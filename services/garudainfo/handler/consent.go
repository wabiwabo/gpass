package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/garudapass/gpass/services/garudainfo/store"
)

// ConsentHandler handles HTTP requests for consent management.
type ConsentHandler struct {
	store store.ConsentStore
}

// NewConsentHandler creates a new ConsentHandler.
func NewConsentHandler(s store.ConsentStore) *ConsentHandler {
	return &ConsentHandler{store: s}
}

type grantRequest struct {
	UserID       string   `json:"user_id"`
	ClientID     string   `json:"client_id"`
	ClientName   string   `json:"client_name"`
	Purpose      string   `json:"purpose"`
	Fields       []string `json:"fields"`
	DurationDays int      `json:"duration_days"`
}

type grantResponse struct {
	ConsentID string `json:"consent_id"`
	ExpiresAt string `json:"expires_at"`
}

type listResponse struct {
	Consents []*consentDTO `json:"consents"`
}

type consentDTO struct {
	ID         string          `json:"id"`
	UserID     string          `json:"user_id"`
	ClientID   string          `json:"client_id"`
	ClientName string          `json:"client_name"`
	Purpose    string          `json:"purpose"`
	Fields     map[string]bool `json:"fields"`
	GrantedAt  string          `json:"granted_at"`
	ExpiresAt  string          `json:"expires_at"`
	Status     string          `json:"status"`
}

type revokeResponse struct {
	Revoked bool `json:"revoked"`
}

// Grant handles POST requests to create a new consent.
func (h *ConsentHandler) Grant(w http.ResponseWriter, r *http.Request) {
	var req grantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	if req.UserID == "" || req.ClientID == "" || len(req.Fields) == 0 || req.DurationDays <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "user_id, client_id, fields, and duration_days are required")
		return
	}

	fields := make(map[string]bool, len(req.Fields))
	for _, f := range req.Fields {
		fields[f] = true
	}

	consent := &store.Consent{
		UserID:          req.UserID,
		ClientID:        req.ClientID,
		ClientName:      req.ClientName,
		Purpose:         req.Purpose,
		Fields:          fields,
		DurationSeconds: int64(req.DurationDays) * 86400,
	}

	if err := h.store.Create(r.Context(), consent); err != nil {
		slog.Error("failed to create consent", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to create consent")
		return
	}

	writeJSON(w, http.StatusCreated, grantResponse{
		ConsentID: consent.ID,
		ExpiresAt: consent.ExpiresAt.Format("2006-01-02T15:04:05Z"),
	})
}

// List handles GET requests to list consents for a user.
func (h *ConsentHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "user_id query parameter is required")
		return
	}

	consents, err := h.store.ListByUser(r.Context(), userID)
	if err != nil {
		slog.Error("failed to list consents", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to list consents")
		return
	}

	dtos := make([]*consentDTO, 0, len(consents))
	for _, c := range consents {
		dtos = append(dtos, &consentDTO{
			ID:         c.ID,
			UserID:     c.UserID,
			ClientID:   c.ClientID,
			ClientName: c.ClientName,
			Purpose:    c.Purpose,
			Fields:     c.Fields,
			GrantedAt:  c.GrantedAt.Format("2006-01-02T15:04:05Z"),
			ExpiresAt:  c.ExpiresAt.Format("2006-01-02T15:04:05Z"),
			Status:     c.Status,
		})
	}

	writeJSON(w, http.StatusOK, listResponse{Consents: dtos})
}

// Revoke handles DELETE requests to revoke a consent by ID.
func (h *ConsentHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "consent id is required")
		return
	}

	err := h.store.Revoke(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrConsentNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "consent not found")
			return
		}
		if errors.Is(err, store.ErrConsentRevoked) {
			writeError(w, http.StatusConflict, "already_revoked", "consent has already been revoked")
			return
		}
		slog.Error("failed to revoke consent", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to revoke consent")
		return
	}

	writeJSON(w, http.StatusOK, revokeResponse{Revoked: true})
}

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Error: code, Message: message})
}
