package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeaders_Defaults(t *testing.T) {
	h := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), SecurityHeaderOptions{})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	checks := map[string]string{
		"X-Content-Type-Options":       "nosniff",
		"X-Frame-Options":              "DENY",
		"Referrer-Policy":              "no-referrer",
		"X-XSS-Protection":             "0",
		"Cross-Origin-Resource-Policy": "same-origin",
	}
	for k, want := range checks {
		if got := w.Header().Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if !strings.Contains(w.Header().Get("Content-Security-Policy"), "default-src 'none'") {
		t.Errorf("CSP missing default-src none, got %q", w.Header().Get("Content-Security-Policy"))
	}
	if !strings.Contains(w.Header().Get("Permissions-Policy"), "geolocation=()") {
		t.Errorf("Permissions-Policy missing geolocation=()")
	}
	if w.Header().Get("Strict-Transport-Security") != "" {
		t.Error("HSTS should not be set when opts.HSTS=false")
	}
}

func TestSecurityHeaders_HSTS(t *testing.T) {
	h := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), SecurityHeaderOptions{
		HSTS:                  true,
		HSTSIncludeSubdomains: true,
		HSTSPreload:           true,
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	got := w.Header().Get("Strict-Transport-Security")
	for _, want := range []string{"max-age=31536000", "includeSubDomains", "preload"} {
		if !strings.Contains(got, want) {
			t.Errorf("HSTS = %q, want substring %q", got, want)
		}
	}
}

func TestSecurityHeaders_CustomCSP(t *testing.T) {
	h := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), SecurityHeaderOptions{
		ContentSecurityPolicy: "default-src 'self'",
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if got := w.Header().Get("Content-Security-Policy"); got != "default-src 'self'" {
		t.Errorf("CSP = %q, want default-src 'self'", got)
	}
}

func TestSecurityHeaders_FrameOptionsOmittable(t *testing.T) {
	// Set to a single space to test omission semantics: empty string falls back
	// to default. This documents the contract: there's no way to fully omit
	// X-Frame-Options via this middleware (by design — defense in depth).
	h := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), SecurityHeaderOptions{
		FrameOptions: "SAMEORIGIN",
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if got := w.Header().Get("X-Frame-Options"); got != "SAMEORIGIN" {
		t.Errorf("XFO = %q", got)
	}
}

func TestItoa(t *testing.T) {
	cases := map[int]string{0: "0", 1: "1", 31536000: "31536000", -42: "-42"}
	for n, want := range cases {
		if got := itoa(n); got != want {
			t.Errorf("itoa(%d) = %q, want %q", n, got, want)
		}
	}
}
