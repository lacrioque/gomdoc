package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestValidateOAuth2Config_RejectsConflictWithBasicAuth(t *testing.T) {
	err := ValidateOAuth2Config(validOAuth2Config(), true)
	if err == nil {
		t.Fatal("expected conflict error")
	}
}

func TestValidateOAuth2Config_RejectsMissingFields(t *testing.T) {
	config := OAuth2Config{ClientID: "client"}
	err := ValidateOAuth2Config(config, false)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "client secret") {
		t.Fatalf("expected missing client secret in error, got %q", err.Error())
	}
}

func TestSignedCookieRoundTripAndTamperRejection(t *testing.T) {
	s := &Server{oauth2Config: OAuth2Config{CookieSecret: "secret"}}

	rec := httptest.NewRecorder()
	s.writeSignedCookie(rec, oauth2SessionCookie, oauth2Session{
		Email:   "user@example.com",
		Expires: time.Now().Add(time.Hour).Unix(),
	}, time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(rec.Result().Cookies()[0])

	session, ok := s.readOAuth2Session(req)
	if !ok {
		t.Fatal("expected valid session")
	}
	if session.Email != "user@example.com" {
		t.Fatalf("expected email user@example.com, got %q", session.Email)
	}

	tampered := *rec.Result().Cookies()[0]
	tampered.Value = strings.Replace(tampered.Value, ".", "x.", 1)
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&tampered)
	if _, ok := s.readOAuth2Session(req); ok {
		t.Fatal("expected tampered cookie to be rejected")
	}
}

func TestSignedCookieRejectsExpiredSession(t *testing.T) {
	s := &Server{oauth2Config: OAuth2Config{CookieSecret: "secret"}}

	rec := httptest.NewRecorder()
	s.writeSignedCookie(rec, oauth2SessionCookie, oauth2Session{
		Email:   "user@example.com",
		Expires: time.Now().Add(-time.Hour).Unix(),
	}, time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(rec.Result().Cookies()[0])
	if _, ok := s.readOAuth2Session(req); ok {
		t.Fatal("expected expired session to be rejected")
	}
}

func TestOAuth2Middleware_RedirectsHTMLRequest(t *testing.T) {
	s := &Server{oauth2Config: validOAuth2Config()}
	handler := s.oauth2Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/docs/page.md?x=1", nil)
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rec.Code)
	}
	location := rec.Header().Get("Location")
	if !strings.HasPrefix(location, "/oauth2/login?next=") {
		t.Fatalf("expected login redirect, got %q", location)
	}
}

func TestOAuth2Middleware_RejectsAPIRequest(t *testing.T) {
	s := &Server{oauth2Config: validOAuth2Config()}
	handler := s.oauth2Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=docs", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestOAuth2Middleware_AllowsValidSession(t *testing.T) {
	s := &Server{oauth2Config: validOAuth2Config()}
	rec := httptest.NewRecorder()
	s.writeSignedCookie(rec, oauth2SessionCookie, oauth2Session{
		Email:   "user@example.com",
		Expires: time.Now().Add(time.Hour).Unix(),
	}, time.Hour)

	handler := s.oauth2Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(rec.Result().Cookies()[0])
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestOAuth2Middleware_BypassesMCPBearerAuthPath(t *testing.T) {
	s := &Server{oauth2Config: validOAuth2Config()}
	handler := s.oauth2Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/mcp/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestOAuth2Callback_SetsSessionForAllowedEmail(t *testing.T) {
	verified := true
	s := testOAuth2Server(t, userInfoResponse{
		Email:         "user@example.com",
		EmailVerified: &verified,
	})

	rec := httptest.NewRecorder()
	s.writeSignedCookie(rec, oauth2StateCookie, oauth2State{
		State:   "state",
		Next:    "/docs/page.md",
		Expires: time.Now().Add(time.Hour).Unix(),
	}, time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/oauth2/callback?state=state&code=good", nil)
	req.AddCookie(rec.Result().Cookies()[0])
	rec = httptest.NewRecorder()
	s.handleOAuth2Callback(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Location") != "/docs/page.md" {
		t.Fatalf("expected redirect to original path, got %q", rec.Header().Get("Location"))
	}

	var foundSession bool
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == oauth2SessionCookie && cookie.Value != "" {
			foundSession = true
		}
	}
	if !foundSession {
		t.Fatal("expected session cookie")
	}
}

func TestOAuth2Callback_RejectsBadState(t *testing.T) {
	s := &Server{oauth2Config: validOAuth2Config()}
	rec := httptest.NewRecorder()
	s.writeSignedCookie(rec, oauth2StateCookie, oauth2State{
		State:   "state",
		Next:    "/",
		Expires: time.Now().Add(time.Hour).Unix(),
	}, time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/oauth2/callback?state=wrong&code=good", nil)
	req.AddCookie(rec.Result().Cookies()[0])
	rec = httptest.NewRecorder()
	s.handleOAuth2Callback(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestOAuth2Callback_RejectsUnverifiedEmail(t *testing.T) {
	verified := false
	s := testOAuth2Server(t, userInfoResponse{
		Email:         "user@example.com",
		EmailVerified: &verified,
	})

	rec := httptest.NewRecorder()
	s.writeSignedCookie(rec, oauth2StateCookie, oauth2State{
		State:   "state",
		Next:    "/",
		Expires: time.Now().Add(time.Hour).Unix(),
	}, time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/oauth2/callback?state=state&code=good", nil)
	req.AddCookie(rec.Result().Cookies()[0])
	rec = httptest.NewRecorder()
	s.handleOAuth2Callback(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestOAuth2Callback_RejectsDisallowedEmail(t *testing.T) {
	s := testOAuth2Server(t, userInfoResponse{Email: "other@example.net"})

	rec := httptest.NewRecorder()
	s.writeSignedCookie(rec, oauth2StateCookie, oauth2State{
		State:   "state",
		Next:    "/",
		Expires: time.Now().Add(time.Hour).Unix(),
	}, time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/oauth2/callback?state=state&code=good", nil)
	req.AddCookie(rec.Result().Cookies()[0])
	rec = httptest.NewRecorder()
	s.handleOAuth2Callback(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestOAuth2Callback_RejectsTokenExchangeFailure(t *testing.T) {
	s := testOAuth2Server(t, userInfoResponse{Email: "user@example.com"})

	rec := httptest.NewRecorder()
	s.writeSignedCookie(rec, oauth2StateCookie, oauth2State{
		State:   "state",
		Next:    "/",
		Expires: time.Now().Add(time.Hour).Unix(),
	}, time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/oauth2/callback?state=state&code=bad", nil)
	req.AddCookie(rec.Result().Cookies()[0])
	rec = httptest.NewRecorder()
	s.handleOAuth2Callback(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func validOAuth2Config() OAuth2Config {
	return OAuth2Config{
		ClientID:       "client",
		ClientSecret:   "secret",
		AuthURL:        "https://provider.example/auth",
		TokenURL:       "https://provider.example/token",
		RedirectURL:    "http://localhost:7331/oauth2/callback",
		UserInfoURL:    "https://provider.example/userinfo",
		Scopes:         []string{"openid", "email"},
		AllowedEmails:  []string{"user@example.com"},
		AllowedDomains: []string{"example.org"},
		CookieSecret:   "cookie-secret",
		SessionTTL:     time.Hour,
	}
}

func testOAuth2Server(t *testing.T, userInfo userInfoResponse) *Server {
	t.Helper()

	config := validOAuth2Config()
	config.AuthURL = "https://provider.example/auth"
	config.TokenURL = "https://provider.example/token"
	config.UserInfoURL = "https://provider.example/userinfo"
	return &Server{
		oauth2Config: config.withDefaults(),
		oauth2Client: &http.Client{Transport: fakeOAuth2Transport{
			t:        t,
			userInfo: userInfo,
		}},
	}
}

type fakeOAuth2Transport struct {
	t        *testing.T
	userInfo userInfoResponse
}

func (rt fakeOAuth2Transport) RoundTrip(r *http.Request) (*http.Response, error) {
	switch r.URL.Path {
	case "/token":
		if err := r.ParseForm(); err != nil {
			rt.t.Fatalf("failed to parse form: %v", err)
		}
		if r.Form.Get("code") != "good" {
			return textResponse(http.StatusBadRequest, "bad code"), nil
		}
		return jsonResponse(http.StatusOK, `{"access_token":"access","token_type":"Bearer","expires_in":3600}`), nil
	case "/userinfo":
		if r.Header.Get("Authorization") != "Bearer access" {
			return textResponse(http.StatusUnauthorized, "unauthorized"), nil
		}
		body, err := json.Marshal(rt.userInfo)
		if err != nil {
			rt.t.Fatalf("failed to marshal userinfo: %v", err)
		}
		return jsonResponse(http.StatusOK, string(body)), nil
	default:
		return textResponse(http.StatusNotFound, "not found"), nil
	}
}

func jsonResponse(status int, body string) *http.Response {
	resp := textResponse(status, body)
	resp.Header.Set("Content-Type", "application/json")
	return resp
}

func textResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
