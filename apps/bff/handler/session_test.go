package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/garudapass/gpass/apps/bff/handler"
	"github.com/garudapass/gpass/apps/bff/session"
)

func TestGetSessionUnauthenticated(t *testing.T) {
	store := session.NewInMemoryStore()
	h := handler.NewSessionHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/auth/session", nil)
	w := httptest.NewRecorder()

	h.GetSession(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp handler.SessionResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Authenticated {
		t.Error("expected authenticated=false")
	}
	if resp.User != nil {
		t.Error("expected user=nil")
	}
}

func TestGetSessionAuthenticated(t *testing.T) {
	store := session.NewInMemoryStore()
	h := handler.NewSessionHandler(store)

	sid, _ := store.Create(context.Background(), &session.Data{
		UserID:    "user-456",
		CSRFToken: "csrf-token-123",
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}, 30*time.Minute)

	req := httptest.NewRequest(http.MethodGet, "/auth/session", nil)
	req.AddCookie(&http.Cookie{Name: "gpass_session", Value: sid})
	w := httptest.NewRecorder()

	h.GetSession(w, req)

	var resp handler.SessionResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if !resp.Authenticated {
		t.Error("expected authenticated=true")
	}
	if resp.User.ID != "user-456" {
		t.Errorf("expected user-456, got %s", resp.User.ID)
	}
	if resp.CSRFToken != "csrf-token-123" {
		t.Errorf("expected csrf-token-123, got %s", resp.CSRFToken)
	}

	// Verify security headers on session response
	if cc := w.Header().Get("Cache-Control"); cc == "" {
		t.Error("expected Cache-Control header on session response")
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("expected application/json; charset=utf-8, got %s", ct)
	}
}
