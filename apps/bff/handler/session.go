package handler

import (
	"encoding/json"
	"net/http"

	"github.com/garudapass/gpass/apps/bff/session"
)

type SessionHandler struct {
	store session.Store
}

func NewSessionHandler(store session.Store) *SessionHandler {
	return &SessionHandler{store: store}
}

type SessionResponse struct {
	Authenticated bool      `json:"authenticated"`
	User          *UserInfo `json:"user"`
	CSRFToken     string    `json:"csrf_token,omitempty"`
}

type UserInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name,omitempty"`
	Email    string `json:"email,omitempty"`
	Verified bool   `json:"verified"`
}

func (h *SessionHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		writeJSON(w, http.StatusOK, SessionResponse{Authenticated: false})
		return
	}

	data, err := h.store.Get(r.Context(), cookie.Value)
	if err != nil {
		writeJSON(w, http.StatusOK, SessionResponse{Authenticated: false})
		return
	}

	writeJSON(w, http.StatusOK, SessionResponse{
		Authenticated: true,
		User: &UserInfo{
			ID: data.UserID,
		},
		CSRFToken: data.CSRFToken,
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
