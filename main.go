// Package main is the entry point for the gomdoc application.
// gomdoc is a simple markdown server that renders .md files as HTML.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gomdoc/server"
)

// version is set at build time via -ldflags.
var version = "dev"

func main() {
	port := flag.Int("port", 7331, "Port to run the server on")
	dir := flag.String("dir", ".", "Base directory to serve markdown files from")
	title := flag.String("title", "gomdoc", "Custom title for the documentation site")
	auth := flag.String("auth", "", "Basic auth credentials in user:password format")
	oauth2ClientID := flag.String("oauth2-client-id", "", "OAuth2 client ID")
	oauth2ClientSecret := flag.String("oauth2-client-secret", "", "OAuth2 client secret")
	oauth2AuthURL := flag.String("oauth2-auth-url", "", "OAuth2 authorization endpoint URL")
	oauth2TokenURL := flag.String("oauth2-token-url", "", "OAuth2 token endpoint URL")
	oauth2RedirectURL := flag.String("oauth2-redirect-url", "", "OAuth2 callback redirect URL")
	oauth2UserInfoURL := flag.String("oauth2-userinfo-url", "", "OAuth2 userinfo endpoint URL")
	oauth2Scopes := flag.String("oauth2-scopes", "", "OAuth2 scopes, comma-separated")
	oauth2AllowedEmails := flag.String("oauth2-allowed-emails", "", "Allowed OAuth2 email addresses, comma-separated")
	oauth2AllowedDomains := flag.String("oauth2-allowed-domains", "", "Allowed OAuth2 email domains, comma-separated")
	oauth2CookieSecret := flag.String("oauth2-cookie-secret", "", "Secret used to sign OAuth2 session cookies")
	mcpToken := flag.String("mcp-token", "", "Bearer token for MCP server authentication (auto-generated if empty)")
	mcpNoAuth := flag.Bool("mcp-no-auth", false, "Disable MCP server authentication entirely")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	// Validate auth format if provided
	var authUser, authPass string
	if *auth != "" {
		parts := strings.SplitN(*auth, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			log.Fatalf("Invalid auth format. Use: -auth user:password")
		}
		authUser = parts[0]
		authPass = parts[1]
	}

	oauth2Config := server.OAuth2Config{
		ClientID:       envFallback(*oauth2ClientID, "GOMDOC_OAUTH2_CLIENT_ID"),
		ClientSecret:   envFallback(*oauth2ClientSecret, "GOMDOC_OAUTH2_CLIENT_SECRET"),
		AuthURL:        envFallback(*oauth2AuthURL, "GOMDOC_OAUTH2_AUTH_URL"),
		TokenURL:       envFallback(*oauth2TokenURL, "GOMDOC_OAUTH2_TOKEN_URL"),
		RedirectURL:    envFallback(*oauth2RedirectURL, "GOMDOC_OAUTH2_REDIRECT_URL"),
		UserInfoURL:    envFallback(*oauth2UserInfoURL, "GOMDOC_OAUTH2_USERINFO_URL"),
		Scopes:         splitCSV(envFallback(*oauth2Scopes, "GOMDOC_OAUTH2_SCOPES")),
		AllowedEmails:  splitCSV(envFallback(*oauth2AllowedEmails, "GOMDOC_OAUTH2_ALLOWED_EMAILS")),
		AllowedDomains: splitCSV(envFallback(*oauth2AllowedDomains, "GOMDOC_OAUTH2_ALLOWED_DOMAINS")),
		CookieSecret:   envFallback(*oauth2CookieSecret, "GOMDOC_OAUTH2_COOKIE_SECRET"),
	}
	if err := server.ValidateOAuth2Config(oauth2Config, authUser != ""); err != nil {
		log.Fatalf("Invalid OAuth2 config: %v", err)
	}

	// Resolve and validate the base directory
	baseDir, err := filepath.Abs(*dir)
	if err != nil {
		log.Fatalf("Error resolving directory path: %v", err)
	}

	info, err := os.Stat(baseDir)
	if err != nil {
		log.Fatalf("Error accessing directory: %v", err)
	}
	if !info.IsDir() {
		log.Fatalf("Path is not a directory: %s", baseDir)
	}

	// Resolve MCP token: use provided, generate, or disable
	resolvedMCPToken := *mcpToken
	if !*mcpNoAuth && resolvedMCPToken == "" {
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			log.Fatalf("Failed to generate MCP token: %v", err)
		}
		resolvedMCPToken = hex.EncodeToString(tokenBytes)
	}
	if *mcpNoAuth {
		resolvedMCPToken = ""
	}

	fmt.Println("gomdoc - Markdown Documentation Server")
	fmt.Println("=======================================")

	srv := server.NewWithAuth(baseDir, *port, *title, authUser, authPass, oauth2Config, resolvedMCPToken, version)
	if err := srv.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func envFallback(value, key string) string {
	if value != "" {
		return value
	}
	return os.Getenv(key)
}

func splitCSV(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
