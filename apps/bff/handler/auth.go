package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/garudapass/gpass/apps/bff/middleware"
	"github.com/garudapass/gpass/apps/bff/session"
)

const (
	cookieName     = "gpass_session"
	csrfCookieName = "gpass_csrf"
	sessionTTL     = 30 * time.Minute
	stateCookieMax = 300 // 5 minutes
)

// AuthConfig holds OIDC and session configuration for the auth handler.
type AuthConfig struct {
	IssuerURL    string
	ClientID     string
	RedirectURI  string
	FrontendURL  string
	CookieDomain string
	SecureCookie bool // true in production/staging
}

// AuthHandler manages the OAuth2/OIDC authorization code flow with PKCE.
type AuthHandler struct {
	cfg   AuthConfig
	store session.Store
}

func NewAuthHandler(cfg AuthConfig, store session.Store) *AuthHandler {
	return &AuthHandler{cfg: cfg, store: store}
}

// Login initiates the authorization code flow by redirecting to Keycloak
// with PKCE (S256) code challenge and random state parameter.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	state, err := randomString(32)
	if err != nil {
		slog.Error("failed to generate state", "error", err, "request_id", middleware.GetRequestID(r.Context()))
		writeErrorJSON(w, http.StatusInternalServerError, "internal_error", "Failed to initiate login")
		return
	}

	verifier, err := randomString(64)
	if err != nil {
		slog.Error("failed to generate PKCE verifier", "error", err, "request_id", middleware.GetRequestID(r.Context()))
		writeErrorJSON(w, http.StatusInternalServerError, "internal_error", "Failed to initiate login")
		return
	}

	challenge := computeCodeChallenge(verifier)

	// Store PKCE verifier in a short-lived HttpOnly cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "gpass_pkce",
		Value:    verifier,
		Path:     "/auth",
		MaxAge:   stateCookieMax,
		HttpOnly: true,
		Secure:   h.cfg.SecureCookie,
		SameSite: http.SameSiteLaxMode,
	})

	// Store state for CSRF validation in callback
	http.SetCookie(w, &http.Cookie{
		Name:     "gpass_state",
		Value:    state,
		Path:     "/auth",
		MaxAge:   stateCookieMax,
		HttpOnly: true,
		Secure:   h.cfg.SecureCookie,
		SameSite: http.SameSiteLaxMode,
	})

	authURL := fmt.Sprintf("%s/protocol/openid-connect/auth?"+
		"response_type=code"+
		"&client_id=%s"+
		"&redirect_uri=%s"+
		"&scope=openid+profile+email"+
		"&state=%s"+
		"&code_challenge=%s"+
		"&code_challenge_method=S256",
		h.cfg.IssuerURL,
		url.QueryEscape(h.cfg.ClientID),
		url.QueryEscape(h.cfg.RedirectURI),
		url.QueryEscape(state),
		url.QueryEscape(challenge),
	)

	slog.Info("login initiated",
		"request_id", middleware.GetRequestID(r.Context()),
		"remote_addr", r.RemoteAddr,
	)

	http.Redirect(w, r, authURL, http.StatusFound)
}

// Callback handles the OAuth2 authorization code callback.
// It validates the state parameter, then exchanges the code for tokens.
func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check for error response from IdP
	if errCode := r.URL.Query().Get("error"); errCode != "" {
		errDesc := r.URL.Query().Get("error_description")
		slog.Warn("callback received error from IdP",
			"error", errCode,
			"description", errDesc,
			"request_id", middleware.GetRequestID(ctx),
		)
		clearAuthCookies(w)
		http.Redirect(w, r, h.cfg.FrontendURL+"?error=auth_failed", http.StatusFound)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		writeErrorJSON(w, http.StatusBadRequest, "missing_code", "Missing authorization code")
		return
	}

	// Validate state parameter matches the cookie (CSRF protection)
	stateParam := r.URL.Query().Get("state")
	stateCookie, err := r.Cookie("gpass_state")
	if err != nil || stateCookie.Value == "" {
		slog.Warn("callback missing state cookie", "request_id", middleware.GetRequestID(ctx))
		writeErrorJSON(w, http.StatusBadRequest, "invalid_state", "State validation failed")
		return
	}
	if subtle.ConstantTimeCompare([]byte(stateParam), []byte(stateCookie.Value)) != 1 {
		slog.Warn("callback state mismatch",
			"request_id", middleware.GetRequestID(ctx),
			"remote_addr", r.RemoteAddr,
		)
		writeErrorJSON(w, http.StatusBadRequest, "state_mismatch", "State parameter does not match")
		return
	}

	// TODO: Token exchange will be implemented in Plan 2 with full OIDC client
	// For now, create a stub session
	csrfToken := mustRandomString(32)
	sess := &session.Data{
		UserID:    "stub-user",
		CSRFToken: csrfToken,
		ExpiresAt: time.Now().Add(sessionTTL),
		UserAgent: r.UserAgent(),
	}

	sid, err := h.store.Create(ctx, sess, sessionTTL)
	if err != nil {
		slog.Error("session creation failed", "error", err, "request_id", middleware.GetRequestID(ctx))
		writeErrorJSON(w, http.StatusInternalServerError, "session_error", "Failed to create session")
		return
	}

	// Set session cookie (HttpOnly, SameSite=Strict)
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    sid,
		Path:     "/",
		Domain:   h.cfg.CookieDomain,
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   h.cfg.SecureCookie,
		SameSite: http.SameSiteStrictMode,
	})

	// Set CSRF cookie (readable by JavaScript for double-submit pattern)
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    csrfToken,
		Path:     "/",
		Domain:   h.cfg.CookieDomain,
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: false, // JS needs to read this for X-CSRF-Token header
		Secure:   h.cfg.SecureCookie,
		SameSite: http.SameSiteStrictMode,
	})

	clearAuthCookies(w)

	slog.Info("login successful",
		"user_id", sess.UserID,
		"request_id", middleware.GetRequestID(ctx),
	)

	http.Redirect(w, r, h.cfg.FrontendURL, http.StatusFound)
}

// Logout destroys the session and clears all auth cookies.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(cookieName)
	if err == nil && cookie.Value != "" {
		_ = h.store.Delete(r.Context(), cookie.Value)
		slog.Info("user logged out", "request_id", middleware.GetRequestID(r.Context()))
	}

	// Clear session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		Domain:   h.cfg.CookieDomain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cfg.SecureCookie,
		SameSite: http.SameSiteStrictMode,
	})

	// Clear CSRF cookie
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    "",
		Path:     "/",
		Domain:   h.cfg.CookieDomain,
		MaxAge:   -1,
		HttpOnly: false,
		Secure:   h.cfg.SecureCookie,
		SameSite: http.SameSiteStrictMode,
	})

	http.Redirect(w, r, h.cfg.FrontendURL, http.StatusFound)
}

func computeCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b)[:n], nil
}

func mustRandomString(n int) string {
	s, err := randomString(n)
	if err != nil {
		panic(err)
	}
	return s
}

func clearAuthCookies(w http.ResponseWriter) {
	for _, name := range []string{"gpass_pkce", "gpass_state"} {
		http.SetCookie(w, &http.Cookie{
			Name:   name,
			Value:  "",
			Path:   "/auth",
			MaxAge: -1,
		})
	}
}

func writeErrorJSON(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error":%q,"message":%q}`, code, message)
}
