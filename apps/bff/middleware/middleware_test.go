package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/apps/bff/middleware"
)

func TestSecurityHeadersPresent(t *testing.T) {
	handler := middleware.SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	expectedHeaders := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":       "DENY",
		"X-XSS-Protection":      "1; mode=block",
		"Cache-Control":          "no-store, no-cache, must-revalidate, private",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}

	for name, expected := range expectedHeaders {
		got := w.Header().Get(name)
		if got != expected {
			t.Errorf("header %s: got %q, want %q", name, got, expected)
		}
	}
}

func TestRequestIDGenerated(t *testing.T) {
	handler := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := middleware.GetRequestID(r.Context())
		if rid == "" {
			t.Error("expected request ID in context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("X-Request-Id") == "" {
		t.Error("expected X-Request-Id response header")
	}
}

func TestRequestIDPreservedFromUpstream(t *testing.T) {
	handler := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := middleware.GetRequestID(r.Context())
		if rid != "upstream-id-123" {
			t.Errorf("expected upstream-id-123, got %s", rid)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-Id", "upstream-id-123")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("X-Request-Id") != "upstream-id-123" {
		t.Error("expected X-Request-Id to be preserved from upstream")
	}
}

func TestRecoveryCatchesPanics(t *testing.T) {
	handler := middleware.Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	// Should not panic
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestAccessLogSetsStatusCode(t *testing.T) {
	handler := middleware.AccessLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
}
