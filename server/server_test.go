package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBearerAuthMiddleware_RejectsNoToken(t *testing.T) {
	s := &Server{mcpToken: "test-secret"}
	handler := s.bearerAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/mcp/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestBearerAuthMiddleware_RejectsWrongToken(t *testing.T) {
	s := &Server{mcpToken: "test-secret"}
	handler := s.bearerAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/mcp/", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestBearerAuthMiddleware_AcceptsCorrectBearer(t *testing.T) {
	s := &Server{mcpToken: "test-secret"}
	handler := s.bearerAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/mcp/", nil)
	req.Header.Set("Authorization", "Bearer test-secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestBearerAuthMiddleware_AcceptsQueryParam(t *testing.T) {
	s := &Server{mcpToken: "test-secret"}
	handler := s.bearerAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/mcp/?token=test-secret", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestBearerAuthMiddleware_RejectsWrongQueryParam(t *testing.T) {
	s := &Server{mcpToken: "test-secret"}
	handler := s.bearerAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/mcp/?token=wrong", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestBearerAuthMiddleware_HeaderTakesPrecedence(t *testing.T) {
	s := &Server{mcpToken: "test-secret"}
	handler := s.bearerAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Correct header, wrong query param — should succeed (header wins)
	req := httptest.NewRequest(http.MethodGet, "/mcp/?token=wrong", nil)
	req.Header.Set("Authorization", "Bearer test-secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 (header takes precedence), got %d", rec.Code)
	}
}
