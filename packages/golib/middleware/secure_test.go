package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecureHeaders(t *testing.T) {
	handler := SecureHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	expectedHeaders := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":       "DENY",
		"X-XSS-Protection":      "0",
		"Referrer-Policy":       "strict-origin-when-cross-origin",
		"Cache-Control":         "no-store",
	}

	for header, expected := range expectedHeaders {
		if got := w.Header().Get(header); got != expected {
			t.Errorf("%s: expected %q, got %q", header, expected, got)
		}
	}

	if csp := w.Header().Get("Content-Security-Policy"); csp == "" {
		t.Error("Content-Security-Policy should be set")
	}
	if pp := w.Header().Get("Permissions-Policy"); pp == "" {
		t.Error("Permissions-Policy should be set")
	}
}

func TestHSTS(t *testing.T) {
	handler := HSTS(63072000)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	hsts := w.Header().Get("Strict-Transport-Security")
	if hsts == "" {
		t.Error("HSTS header should be set")
	}
	if hsts != "max-age=63072000; includeSubDomains; preload" {
		t.Errorf("unexpected HSTS value: %s", hsts)
	}
}

func TestSecureHeaders_PassesThrough(t *testing.T) {
	handler := SecureHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "test")
		w.WriteHeader(http.StatusTeapot)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusTeapot {
		t.Errorf("expected 418, got %d", w.Code)
	}
	if w.Header().Get("X-Custom") != "test" {
		t.Error("custom header should pass through")
	}
}
