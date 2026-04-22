package server

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

const (
	oauth2StateCookie   = "gomdoc_oauth2_state"
	oauth2SessionCookie = "gomdoc_oauth2_session"
	defaultSessionTTL   = 24 * time.Hour
	stateTTL            = 10 * time.Minute
)

// OAuth2Config contains provider-agnostic OAuth2 authentication settings.
type OAuth2Config struct {
	ClientID       string
	ClientSecret   string
	AuthURL        string
	TokenURL       string
	RedirectURL    string
	UserInfoURL    string
	Scopes         []string
	AllowedEmails  []string
	AllowedDomains []string
	CookieSecret   string
	SessionTTL     time.Duration
}

type oauth2State struct {
	State   string `json:"state"`
	Next    string `json:"next"`
	Expires int64  `json:"expires"`
}

type oauth2Session struct {
	Email   string `json:"email"`
	Expires int64  `json:"expires"`
}

type userInfoResponse struct {
	Email         string `json:"email"`
	EmailVerified *bool  `json:"email_verified,omitempty"`
}

func (c OAuth2Config) Enabled() bool {
	return c.ClientID != "" ||
		c.ClientSecret != "" ||
		c.AuthURL != "" ||
		c.TokenURL != "" ||
		c.RedirectURL != "" ||
		c.UserInfoURL != "" ||
		c.CookieSecret != "" ||
		len(c.Scopes) > 0 ||
		len(c.AllowedEmails) > 0 ||
		len(c.AllowedDomains) > 0
}

func (c OAuth2Config) withDefaults() OAuth2Config {
	if c.SessionTTL == 0 {
		c.SessionTTL = defaultSessionTTL
	}
	c.AllowedEmails = normalizeList(c.AllowedEmails)
	c.AllowedDomains = normalizeDomains(c.AllowedDomains)
	c.Scopes = normalizeList(c.Scopes)
	return c
}

// ValidateOAuth2Config returns an error for incomplete or conflicting auth config.
func ValidateOAuth2Config(config OAuth2Config, basicAuthEnabled bool) error {
	config = config.withDefaults()
	if !config.Enabled() {
		return nil
	}
	if basicAuthEnabled {
		return errors.New("cannot enable both -auth and OAuth2 authentication")
	}
	missing := make([]string, 0)
	if config.ClientID == "" {
		missing = append(missing, "client ID")
	}
	if config.ClientSecret == "" {
		missing = append(missing, "client secret")
	}
	if config.AuthURL == "" {
		missing = append(missing, "auth URL")
	}
	if config.TokenURL == "" {
		missing = append(missing, "token URL")
	}
	if config.RedirectURL == "" {
		missing = append(missing, "redirect URL")
	}
	if config.UserInfoURL == "" {
		missing = append(missing, "userinfo URL")
	}
	if config.CookieSecret == "" {
		missing = append(missing, "cookie secret")
	}
	if len(config.AllowedEmails) == 0 && len(config.AllowedDomains) == 0 {
		missing = append(missing, "allowed emails or allowed domains")
	}
	if len(missing) > 0 {
		return fmt.Errorf("incomplete OAuth2 config: missing %s", strings.Join(missing, ", "))
	}
	return nil
}

func (s *Server) oauth2Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.isOAuth2BypassPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		if _, ok := s.readOAuth2Session(r); ok {
			next.ServeHTTP(w, r)
			return
		}

		if wantsHTML(r) {
			target := "/oauth2/login?next=" + url.QueryEscape(nextPath(r))
			http.Redirect(w, r, target, http.StatusFound)
			return
		}
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}

func (s *Server) isOAuth2BypassPath(path string) bool {
	return path == "/oauth2/login" ||
		path == "/oauth2/callback" ||
		path == "/oauth2/logout" ||
		strings.HasPrefix(path, "/static/") ||
		strings.HasPrefix(path, "/mcp/")
}

func (s *Server) handleOAuth2Login(w http.ResponseWriter, r *http.Request) {
	if !s.oauth2Config.Enabled() {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state, err := randomHex(32)
	if err != nil {
		http.Error(w, "Failed to create OAuth2 state", http.StatusInternalServerError)
		return
	}

	next := sanitizeNext(r.URL.Query().Get("next"))
	if next == "" {
		next = "/"
	}
	s.writeSignedCookie(w, oauth2StateCookie, oauth2State{
		State:   state,
		Next:    next,
		Expires: time.Now().Add(stateTTL).Unix(),
	}, stateTTL)

	http.Redirect(w, r, s.oauth2ProviderConfig().AuthCodeURL(state), http.StatusFound)
}

func (s *Server) handleOAuth2Callback(w http.ResponseWriter, r *http.Request) {
	if !s.oauth2Config.Enabled() {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if errText := r.URL.Query().Get("error"); errText != "" {
		http.Error(w, "OAuth2 authorization failed", http.StatusUnauthorized)
		return
	}

	var stored oauth2State
	if !s.readSignedCookie(r, oauth2StateCookie, &stored) ||
		stored.Expires < time.Now().Unix() ||
		stored.State == "" ||
		!hmac.Equal([]byte(stored.State), []byte(r.URL.Query().Get("state"))) {
		http.Error(w, "Invalid OAuth2 state", http.StatusUnauthorized)
		return
	}
	s.clearCookie(w, oauth2StateCookie)

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Missing OAuth2 code", http.StatusUnauthorized)
		return
	}

	token, err := s.oauth2ProviderConfig().Exchange(s.oauth2Context(r.Context()), code)
	if err != nil || token == nil || !token.Valid() {
		http.Error(w, "OAuth2 token exchange failed", http.StatusUnauthorized)
		return
	}

	email, err := s.fetchOAuth2Email(r.Context(), token)
	if err != nil {
		http.Error(w, "OAuth2 userinfo rejected", http.StatusUnauthorized)
		return
	}
	if !s.isAllowedOAuth2Email(email) {
		http.Error(w, "OAuth2 email is not allowed", http.StatusForbidden)
		return
	}

	s.writeSignedCookie(w, oauth2SessionCookie, oauth2Session{
		Email:   strings.ToLower(strings.TrimSpace(email)),
		Expires: time.Now().Add(s.oauth2Config.SessionTTL).Unix(),
	}, s.oauth2Config.SessionTTL)
	http.Redirect(w, r, sanitizeNext(stored.Next), http.StatusFound)
}

func (s *Server) handleOAuth2Logout(w http.ResponseWriter, r *http.Request) {
	if !s.oauth2Config.Enabled() {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.clearCookie(w, oauth2SessionCookie)
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) oauth2ProviderConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     s.oauth2Config.ClientID,
		ClientSecret: s.oauth2Config.ClientSecret,
		RedirectURL:  s.oauth2Config.RedirectURL,
		Scopes:       s.oauth2Config.Scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  s.oauth2Config.AuthURL,
			TokenURL: s.oauth2Config.TokenURL,
		},
	}
}

func (s *Server) oauth2Context(ctx context.Context) context.Context {
	if s.oauth2Client == nil {
		return ctx
	}
	return context.WithValue(ctx, oauth2.HTTPClient, s.oauth2Client)
}

func (s *Server) httpClient() *http.Client {
	if s.oauth2Client != nil {
		return s.oauth2Client
	}
	return http.DefaultClient
}

func (s *Server) fetchOAuth2Email(ctx context.Context, token *oauth2.Token) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.oauth2Config.UserInfoURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("userinfo returned %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}

	var info userInfoResponse
	if err := json.Unmarshal(body, &info); err != nil {
		return "", err
	}
	email := strings.ToLower(strings.TrimSpace(info.Email))
	if email == "" {
		return "", errors.New("userinfo response did not include email")
	}
	if info.EmailVerified != nil && !*info.EmailVerified {
		return "", errors.New("userinfo email is not verified")
	}
	return email, nil
}

func (s *Server) isAllowedOAuth2Email(email string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	for _, allowed := range s.oauth2Config.AllowedEmails {
		if email == allowed {
			return true
		}
	}
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return false
	}
	domain := email[at+1:]
	for _, allowedDomain := range s.oauth2Config.AllowedDomains {
		if domain == allowedDomain {
			return true
		}
	}
	return false
}

func (s *Server) readOAuth2Session(r *http.Request) (oauth2Session, bool) {
	var session oauth2Session
	if !s.readSignedCookie(r, oauth2SessionCookie, &session) {
		return session, false
	}
	if session.Email == "" || session.Expires < time.Now().Unix() {
		return session, false
	}
	return session, true
}

func (s *Server) writeSignedCookie(w http.ResponseWriter, name string, value any, maxAge time.Duration) {
	payload, err := json.Marshal(value)
	if err != nil {
		return
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(s.oauth2Config.CookieSecret))
	mac.Write([]byte(encoded))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    encoded + "." + signature,
		Path:     "/",
		MaxAge:   int(maxAge.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) readSignedCookie(r *http.Request, name string, dest any) bool {
	cookie, err := r.Cookie(name)
	if err != nil {
		return false
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 {
		return false
	}
	mac := hmac.New(sha256.New, []byte(s.oauth2Config.CookieSecret))
	mac.Write([]byte(parts[0]))
	expected := mac.Sum(nil)
	actual, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(actual, expected) {
		return false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	return json.Unmarshal(payload, dest) == nil
}

func (s *Server) clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func wantsHTML(r *http.Request) bool {
	if r.URL.Path == "/api/search" || strings.HasPrefix(r.URL.Path, "/mcp/") {
		return false
	}
	accept := r.Header.Get("Accept")
	return accept == "" || strings.Contains(accept, "text/html")
}

func nextPath(r *http.Request) string {
	if r.URL.RawQuery == "" {
		return r.URL.Path
	}
	return r.URL.Path + "?" + r.URL.RawQuery
}

func sanitizeNext(next string) string {
	if next == "" {
		return "/"
	}
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		return "/"
	}
	return next
}

func randomHex(size int) (string, error) {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func normalizeList(values []string) []string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" {
			normalized = append(normalized, value)
		}
	}
	return normalized
}

func normalizeDomains(values []string) []string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(value, "@")))
		if value != "" {
			normalized = append(normalized, value)
		}
	}
	return normalized
}
