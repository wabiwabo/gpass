package mwhsts

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxAge != 63072000 {
		t.Errorf("MaxAge: got %d", cfg.MaxAge)
	}
	if !cfg.IncludeSubDomains {
		t.Error("IncludeSubDomains should be true")
	}
	if !cfg.Preload {
		t.Error("Preload should be true")
	}
}

func TestSimple(t *testing.T) {
	handler := Simple(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	hsts := rr.Header().Get("Strict-Transport-Security")
	if !strings.Contains(hsts, "max-age=63072000") {
		t.Errorf("should contain max-age: %s", hsts)
	}
	if !strings.Contains(hsts, "includeSubDomains") {
		t.Errorf("should contain includeSubDomains: %s", hsts)
	}
	if !strings.Contains(hsts, "preload") {
		t.Errorf("should contain preload: %s", hsts)
	}
}

func TestCustomConfig(t *testing.T) {
	handler := Middleware(Config{MaxAge: 3600})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	hsts := rr.Header().Get("Strict-Transport-Security")
	if !strings.Contains(hsts, "max-age=3600") {
		t.Errorf("should contain custom max-age: %s", hsts)
	}
	if strings.Contains(hsts, "includeSubDomains") {
		t.Error("should not contain includeSubDomains")
	}
	if strings.Contains(hsts, "preload") {
		t.Error("should not contain preload")
	}
}

func TestZeroMaxAge(t *testing.T) {
	handler := Middleware(Config{MaxAge: 0})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	hsts := rr.Header().Get("Strict-Transport-Security")
	if !strings.Contains(hsts, "63072000") {
		t.Errorf("zero max-age should use default: %s", hsts)
	}
}

func TestMiddlewarePassthrough(t *testing.T) {
	handler := Simple(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "yes")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("body"))
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("status: got %d", rr.Code)
	}
	if rr.Header().Get("X-Custom") != "yes" {
		t.Error("custom header should be preserved")
	}
	if rr.Body.String() != "body" {
		t.Errorf("body: got %q", rr.Body.String())
	}
}
