package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/garudapass/gpass/apps/bff/middleware"
	"github.com/garudapass/gpass/apps/bff/session"
)

func TestRequireSessionRejectsNoCookie(t *testing.T) {
	store := session.NewInMemoryStore()
	handler := middleware.RequireSession(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "unauthorized") {
		t.Errorf("expected unauthorized error, got: %s", w.Body.String())
	}
}

func TestRequireSessionRejectsInvalidSession(t *testing.T) {
	store := session.NewInMemoryStore()
	handler := middleware.RequireSession(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.AddCookie(&http.Cookie{Name: "gpass_session", Value: "nonexistent-session-id"})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestRequireSessionPassesValidSession(t *testing.T) {
	store := session.NewInMemoryStore()
	sid, _ := store.Create(context.Background(), &session.Data{
		UserID:    "user-789",
		CSRFToken: "csrf-123",
		ExpiresAt: time.Now().Add(30 * time.Minute),
	}, 30*time.Minute)

	handler := middleware.RequireSession(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := middleware.GetSessionData(r.Context())
		if data == nil {
			t.Error("expected session data in context")
			return
		}
		if data.UserID != "user-789" {
			t.Errorf("expected user-789, got %s", data.UserID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.AddCookie(&http.Cookie{Name: "gpass_session", Value: sid})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestRequireSessionRejectsExpired(t *testing.T) {
	store := session.NewInMemoryStore()
	sid, _ := store.Create(context.Background(), &session.Data{
		UserID:    "user-expired",
		ExpiresAt: time.Now().Add(-1 * time.Minute), // Already expired
	}, 30*time.Minute)

	handler := middleware.RequireSession(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.AddCookie(&http.Cookie{Name: "gpass_session", Value: sid})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for expired session, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "session_expired") {
		t.Errorf("expected session_expired error, got: %s", w.Body.String())
	}
}

func TestGetSessionDataNilWhenAbsent(t *testing.T) {
	data := middleware.GetSessionData(context.Background())
	if data != nil {
		t.Error("expected nil when no session in context")
	}
}

func TestHSTSEnabledWhenConfigured(t *testing.T) {
	handler := middleware.SecurityHeadersWithOptions(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), true)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	hsts := w.Header().Get("Strict-Transport-Security")
	if hsts == "" {
		t.Error("expected HSTS header when enabled")
	}
	if !strings.Contains(hsts, "max-age=31536000") {
		t.Errorf("expected 1-year max-age, got: %s", hsts)
	}
	if !strings.Contains(hsts, "includeSubDomains") {
		t.Errorf("expected includeSubDomains, got: %s", hsts)
	}
}

func TestHSTSDisabledByDefault(t *testing.T) {
	handler := middleware.SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("Strict-Transport-Security") != "" {
		t.Error("HSTS should not be set when disabled")
	}
}
