package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/garudapass/gpass/apps/bff/middleware"
)

func TestCSRFRejectsPostWithoutToken(t *testing.T) {
	handler := middleware.CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}

	// Should return structured JSON error
	if !strings.Contains(w.Body.String(), "missing_csrf_token") {
		t.Errorf("expected structured error, got: %s", w.Body.String())
	}
}

func TestCSRFAllowsGetWithoutToken(t *testing.T) {
	handler := middleware.CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCSRFAllowsPostWithValidToken(t *testing.T) {
	handler := middleware.CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	token := "valid-csrf-token-with-sufficient-length"
	req := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	req.Header.Set("X-CSRF-Token", token)
	req.AddCookie(&http.Cookie{Name: "gpass_csrf", Value: token})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCSRFRejectsMismatchedToken(t *testing.T) {
	handler := middleware.CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	req.Header.Set("X-CSRF-Token", "token-a")
	req.AddCookie(&http.Cookie{Name: "gpass_csrf", Value: "token-b"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "csrf_mismatch") {
		t.Errorf("expected mismatch error, got: %s", w.Body.String())
	}
}

func TestCSRFAllowsHEADAndOPTIONS(t *testing.T) {
	handler := middleware.CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, method := range []string{http.MethodHead, http.MethodOptions} {
		req := httptest.NewRequest(method, "/api/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("%s: expected 200, got %d", method, w.Code)
		}
	}
}
