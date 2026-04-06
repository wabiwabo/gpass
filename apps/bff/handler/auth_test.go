package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/garudapass/gpass/apps/bff/handler"
	"github.com/garudapass/gpass/apps/bff/session"
)

func newTestAuthHandler() *handler.AuthHandler {
	store := session.NewInMemoryStore()
	return handler.NewAuthHandler(handler.AuthConfig{
		IssuerURL:    "http://keycloak:8080/realms/garudapass",
		ClientID:     "bff-client",
		RedirectURI:  "http://localhost:4000/auth/callback",
		FrontendURL:  "http://localhost:3000",
		SecureCookie: false,
	}, store)
}

func TestLoginRedirect(t *testing.T) {
	h := newTestAuthHandler()

	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", w.Code)
	}

	loc := w.Header().Get("Location")
	if loc == "" {
		t.Fatal("expected Location header")
	}

	// Verify the redirect URL contains required OAuth2 PKCE params
	for _, param := range []string{"code_challenge", "code_challenge_method=S256", "state=", "client_id=", "response_type=code"} {
		if !strings.Contains(loc, param) {
			t.Errorf("redirect URL missing %s: %s", param, loc)
		}
	}
}

func TestLoginSetsPKCEAndStateCookies(t *testing.T) {
	h := newTestAuthHandler()

	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	w := httptest.NewRecorder()

	h.Login(w, req)

	cookies := w.Result().Cookies()
	hasPKCE := false
	hasState := false
	for _, c := range cookies {
		if c.Name == "gpass_pkce" {
			hasPKCE = true
			if !c.HttpOnly {
				t.Error("gpass_pkce should be HttpOnly")
			}
		}
		if c.Name == "gpass_state" {
			hasState = true
			if !c.HttpOnly {
				t.Error("gpass_state should be HttpOnly")
			}
		}
	}
	if !hasPKCE {
		t.Error("expected gpass_pkce cookie")
	}
	if !hasState {
		t.Error("expected gpass_state cookie")
	}
}

func TestCallbackRejectsWithoutCode(t *testing.T) {
	h := newTestAuthHandler()

	req := httptest.NewRequest(http.MethodGet, "/auth/callback", nil)
	w := httptest.NewRecorder()

	h.Callback(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCallbackRejectsStateMismatch(t *testing.T) {
	h := newTestAuthHandler()

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=abc&state=wrong-state", nil)
	req.AddCookie(&http.Cookie{Name: "gpass_state", Value: "correct-state"})
	w := httptest.NewRecorder()

	h.Callback(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for state mismatch, got %d", w.Code)
	}
}

func TestCallbackRejectsMissingStateCookie(t *testing.T) {
	h := newTestAuthHandler()

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=abc&state=some-state", nil)
	w := httptest.NewRecorder()

	h.Callback(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing state cookie, got %d", w.Code)
	}
}

func TestCallbackHandlesIdPError(t *testing.T) {
	h := newTestAuthHandler()

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?error=access_denied&error_description=User+cancelled", nil)
	w := httptest.NewRecorder()

	h.Callback(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if !strings.Contains(loc, "error=auth_failed") {
		t.Errorf("expected error redirect, got: %s", loc)
	}
}

func TestLogout(t *testing.T) {
	h := newTestAuthHandler()

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	w := httptest.NewRecorder()

	h.Logout(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", w.Code)
	}

	cookies := w.Result().Cookies()
	sessionCleared := false
	csrfCleared := false
	for _, c := range cookies {
		if c.Name == "gpass_session" && c.MaxAge < 0 {
			sessionCleared = true
		}
		if c.Name == "gpass_csrf" && c.MaxAge < 0 {
			csrfCleared = true
		}
	}
	if !sessionCleared {
		t.Error("expected gpass_session cookie to be cleared")
	}
	if !csrfCleared {
		t.Error("expected gpass_csrf cookie to be cleared")
	}
}
