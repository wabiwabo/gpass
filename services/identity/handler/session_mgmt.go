package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Session represents an active user session.
type Session struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	DeviceInfo string    `json:"device_info"`
	IPAddress  string    `json:"ip_address"`
	CreatedAt  time.Time `json:"created_at"`
	LastActive time.Time `json:"last_active"`
	ExpiresAt  time.Time `json:"expires_at"`
	Current    bool      `json:"current"`
}

// SessionStore defines the interface for session persistence.
type SessionStore interface {
	List(userID string) ([]*Session, error)
	Revoke(userID, sessionID string) error
	RevokeAll(userID string) (int, error)
}

// SessionManager handles user session management endpoints.
type SessionManager struct {
	sessions SessionStore
}

// NewSessionManager creates a new SessionManager.
func NewSessionManager(sessions SessionStore) *SessionManager {
	return &SessionManager{sessions: sessions}
}

// ListSessions handles GET /api/v1/identity/sessions.
// Returns all active sessions for the authenticated user.
func (m *SessionManager) ListSessions(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "missing_user_id", "X-User-ID header is required")
		return
	}

	sessions, err := m.sessions.List(userID)
	if err != nil {
		slog.Error("failed to list sessions", "error", err, "user_id", userID)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list sessions")
		return
	}

	// Mark the current session based on X-Session-ID header.
	currentSessionID := r.Header.Get("X-Session-ID")
	for _, s := range sessions {
		if s.ID == currentSessionID {
			s.Current = true
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(sessions)
}

// RevokeSession handles DELETE /api/v1/identity/sessions/{id}.
// Revokes a specific session (logout that device).
func (m *SessionManager) RevokeSession(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "missing_user_id", "X-User-ID header is required")
		return
	}

	// Extract session ID from the URL path.
	sessionID := extractSessionID(r.URL.Path)
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "missing_session_id", "Session ID is required in path")
		return
	}

	if err := m.sessions.Revoke(userID, sessionID); err != nil {
		if err.Error() == "session not found" {
			writeError(w, http.StatusNotFound, "not_found", "Session not found")
			return
		}
		slog.Error("failed to revoke session", "error", err, "user_id", userID, "session_id", sessionID)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to revoke session")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "revoked",
	})
}

// RevokeAllSessions handles DELETE /api/v1/identity/sessions.
// Revokes all sessions except the current one (logout everywhere else).
func (m *SessionManager) RevokeAllSessions(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "missing_user_id", "X-User-ID header is required")
		return
	}

	count, err := m.sessions.RevokeAll(userID)
	if err != nil {
		slog.Error("failed to revoke all sessions", "error", err, "user_id", userID)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to revoke sessions")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"status":  "revoked",
		"revoked": count,
	})
}

// extractSessionID extracts the session ID from a path like /api/v1/identity/sessions/{id}.
func extractSessionID(path string) string {
	parts := strings.Split(strings.TrimRight(path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
