package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// InMemorySessionStore implements SessionStore for testing.
type InMemorySessionStore struct {
	mu       sync.RWMutex
	sessions map[string][]*Session // userID -> sessions
}

// NewInMemorySessionStore creates a new in-memory session store.
func NewInMemorySessionStore() *InMemorySessionStore {
	return &InMemorySessionStore{
		sessions: make(map[string][]*Session),
	}
}

// Add inserts a session for testing.
func (s *InMemorySessionStore) Add(session *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.UserID] = append(s.sessions[session.UserID], session)
}

// List returns all sessions for a user.
func (s *InMemorySessionStore) List(userID string) ([]*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions := s.sessions[userID]
	if sessions == nil {
		return []*Session{}, nil
	}

	// Return copies to avoid mutation.
	result := make([]*Session, len(sessions))
	for i, sess := range sessions {
		cp := *sess
		result[i] = &cp
	}
	return result, nil
}

// Revoke removes a specific session.
func (s *InMemorySessionStore) Revoke(userID, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessions := s.sessions[userID]
	for i, sess := range sessions {
		if sess.ID == sessionID {
			s.sessions[userID] = append(sessions[:i], sessions[i+1:]...)
			return nil
		}
	}
	return errors.New("session not found")
}

// RevokeAll removes all sessions for a user and returns how many were removed.
func (s *InMemorySessionStore) RevokeAll(userID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessions := s.sessions[userID]
	count := len(sessions)
	delete(s.sessions, userID)
	return count, nil
}

// --- Store Tests ---

func TestInMemorySessionStore_List(t *testing.T) {
	store := NewInMemorySessionStore()
	now := time.Now()

	store.Add(&Session{ID: "s1", UserID: "user-1", DeviceInfo: "Chrome", CreatedAt: now})
	store.Add(&Session{ID: "s2", UserID: "user-1", DeviceInfo: "Firefox", CreatedAt: now})
	store.Add(&Session{ID: "s3", UserID: "user-2", DeviceInfo: "Safari", CreatedAt: now})

	sessions, err := store.List("user-1")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("len(sessions) = %d, want 2", len(sessions))
	}
}

func TestInMemorySessionStore_Revoke(t *testing.T) {
	store := NewInMemorySessionStore()
	now := time.Now()

	store.Add(&Session{ID: "s1", UserID: "user-1", DeviceInfo: "Chrome", CreatedAt: now})
	store.Add(&Session{ID: "s2", UserID: "user-1", DeviceInfo: "Firefox", CreatedAt: now})

	if err := store.Revoke("user-1", "s1"); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}

	sessions, _ := store.List("user-1")
	if len(sessions) != 1 {
		t.Errorf("len(sessions) = %d, want 1", len(sessions))
	}
	if sessions[0].ID != "s2" {
		t.Errorf("remaining session ID = %q, want %q", sessions[0].ID, "s2")
	}
}

func TestInMemorySessionStore_Revoke_NotFound(t *testing.T) {
	store := NewInMemorySessionStore()

	err := store.Revoke("user-1", "nonexistent")
	if err == nil {
		t.Fatal("Revoke() error = nil, want error for nonexistent session")
	}
}

func TestInMemorySessionStore_RevokeAll(t *testing.T) {
	store := NewInMemorySessionStore()
	now := time.Now()

	store.Add(&Session{ID: "s1", UserID: "user-1", DeviceInfo: "Chrome", CreatedAt: now})
	store.Add(&Session{ID: "s2", UserID: "user-1", DeviceInfo: "Firefox", CreatedAt: now})
	store.Add(&Session{ID: "s3", UserID: "user-1", DeviceInfo: "Safari", CreatedAt: now})

	count, err := store.RevokeAll("user-1")
	if err != nil {
		t.Fatalf("RevokeAll() error = %v", err)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}

	sessions, _ := store.List("user-1")
	if len(sessions) != 0 {
		t.Errorf("len(sessions) = %d, want 0", len(sessions))
	}
}

// --- Handler Tests ---

func newTestSessionManager() (*SessionManager, *InMemorySessionStore) {
	store := NewInMemorySessionStore()
	mgr := NewSessionManager(store)
	return mgr, store
}

func TestListSessions_Success(t *testing.T) {
	mgr, store := newTestSessionManager()
	now := time.Now()

	store.Add(&Session{ID: "s1", UserID: "user-1", DeviceInfo: "Chrome", IPAddress: "1.2.3.4", CreatedAt: now, LastActive: now, ExpiresAt: now.Add(time.Hour)})
	store.Add(&Session{ID: "s2", UserID: "user-1", DeviceInfo: "Firefox", IPAddress: "5.6.7.8", CreatedAt: now, LastActive: now, ExpiresAt: now.Add(time.Hour)})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/identity/sessions", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.Header.Set("X-Session-ID", "s1")
	w := httptest.NewRecorder()

	mgr.ListSessions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var sessions []Session
	if err := json.NewDecoder(w.Body).Decode(&sessions); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(sessions) != 2 {
		t.Fatalf("len(sessions) = %d, want 2", len(sessions))
	}

	// Verify current session is marked.
	var foundCurrent bool
	for _, s := range sessions {
		if s.ID == "s1" && s.Current {
			foundCurrent = true
		}
		if s.ID == "s2" && s.Current {
			t.Error("session s2 should not be marked as current")
		}
	}
	if !foundCurrent {
		t.Error("session s1 should be marked as current")
	}
}

func TestRevokeSession_Success(t *testing.T) {
	mgr, store := newTestSessionManager()
	now := time.Now()

	store.Add(&Session{ID: "s1", UserID: "user-1", DeviceInfo: "Chrome", CreatedAt: now})
	store.Add(&Session{ID: "s2", UserID: "user-1", DeviceInfo: "Firefox", CreatedAt: now})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/identity/sessions/s1", nil)
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()

	mgr.RevokeSession(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "revoked" {
		t.Errorf("status = %q, want %q", resp["status"], "revoked")
	}

	// Verify session was removed.
	sessions, _ := store.List("user-1")
	if len(sessions) != 1 {
		t.Errorf("remaining sessions = %d, want 1", len(sessions))
	}
}

func TestRevokeSession_NotFound(t *testing.T) {
	mgr, _ := newTestSessionManager()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/identity/sessions/nonexistent", nil)
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()

	mgr.RevokeSession(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["error"] != "not_found" {
		t.Errorf("error = %q, want %q", resp["error"], "not_found")
	}
}

func TestRevokeAllSessions_ReturnsCount(t *testing.T) {
	mgr, store := newTestSessionManager()
	now := time.Now()

	store.Add(&Session{ID: "s1", UserID: "user-1", DeviceInfo: "Chrome", CreatedAt: now})
	store.Add(&Session{ID: "s2", UserID: "user-1", DeviceInfo: "Firefox", CreatedAt: now})
	store.Add(&Session{ID: "s3", UserID: "user-1", DeviceInfo: "Safari", CreatedAt: now})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/identity/sessions", nil)
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()

	mgr.RevokeAllSessions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "revoked" {
		t.Errorf("status = %v, want %q", resp["status"], "revoked")
	}
	if revoked, ok := resp["revoked"].(float64); !ok || int(revoked) != 3 {
		t.Errorf("revoked = %v, want 3", resp["revoked"])
	}
}

func TestListSessions_MissingUserID(t *testing.T) {
	mgr, _ := newTestSessionManager()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/identity/sessions", nil)
	w := httptest.NewRecorder()

	mgr.ListSessions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRevokeSession_MissingUserID(t *testing.T) {
	mgr, _ := newTestSessionManager()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/identity/sessions/s1", nil)
	w := httptest.NewRecorder()

	mgr.RevokeSession(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRevokeAllSessions_MissingUserID(t *testing.T) {
	mgr, _ := newTestSessionManager()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/identity/sessions", nil)
	w := httptest.NewRecorder()

	mgr.RevokeAllSessions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
