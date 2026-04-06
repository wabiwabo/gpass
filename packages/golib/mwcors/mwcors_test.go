package mwcors

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddleware_AllowAll(t *testing.T) {
	mw := Middleware(DefaultConfig())
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/api", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("Allow-Origin = %q", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestMiddleware_SpecificOrigin(t *testing.T) {
	cfg := Config{
		AllowedOrigins:   []string{"https://app.example.com"},
		AllowCredentials: true,
	}
	mw := Middleware(cfg)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest("GET", "/api", nil)
	req.Header.Set("Origin", "https://app.example.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "https://app.example.com" {
		t.Errorf("Allow-Origin = %q", w.Header().Get("Access-Control-Allow-Origin"))
	}
	if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Error("should set credentials")
	}
}

func TestMiddleware_BlockedOrigin(t *testing.T) {
	cfg := Config{AllowedOrigins: []string{"https://allowed.com"}}
	mw := Middleware(cfg)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest("GET", "/api", nil)
	req.Header.Set("Origin", "https://evil.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("should not set Allow-Origin for blocked origin")
	}
}

func TestMiddleware_Preflight(t *testing.T) {
	mw := Middleware(DefaultConfig())
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for preflight")
	}))

	req := httptest.NewRequest("OPTIONS", "/api", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Errorf("status = %d, want 204", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("should set Allow-Methods")
	}
}

func TestMiddleware_NoOrigin(t *testing.T) {
	mw := Middleware(DefaultConfig())
	var called bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("should call handler without Origin")
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("should not set CORS headers without Origin")
	}
}

func TestMiddleware_ExposedHeaders(t *testing.T) {
	cfg := Config{
		AllowedOrigins: []string{"*"},
		ExposedHeaders: []string{"X-Request-ID", "X-Total-Count"},
	}
	mw := Middleware(cfg)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Expose-Headers") == "" {
		t.Error("should set exposed headers")
	}
}

func TestMiddleware_VaryHeader(t *testing.T) {
	cfg := Config{
		AllowedOrigins:   []string{"https://app.com"},
		AllowCredentials: true,
	}
	mw := Middleware(cfg)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://app.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("Vary") != "Origin" {
		t.Error("should set Vary: Origin for specific origins")
	}
}
