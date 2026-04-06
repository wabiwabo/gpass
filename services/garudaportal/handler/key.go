package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/garudapass/gpass/services/garudaportal/apikey"
	"github.com/garudapass/gpass/services/garudaportal/store"
)

// KeyHandler handles API key management endpoints.
type KeyHandler struct {
	appStore store.AppStore
	keyStore store.KeyStore
}

// NewKeyHandler creates a new key handler.
func NewKeyHandler(appStore store.AppStore, keyStore store.KeyStore) *KeyHandler {
	return &KeyHandler{
		appStore: appStore,
		keyStore: keyStore,
	}
}

type createKeyRequest struct {
	Name          string `json:"name"`
	ExpiresInDays *int   `json:"expires_in_days"`
}

type keyResponse struct {
	ID           string  `json:"id"`
	KeyPrefix    string  `json:"key_prefix"`
	Name         string  `json:"name"`
	Environment  string  `json:"environment"`
	Status       string  `json:"status"`
	LastUsedAt   *string `json:"last_used_at,omitempty"`
	ExpiresAt    *string `json:"expires_at,omitempty"`
	CreatedAt    string  `json:"created_at"`
	PlaintextKey string  `json:"plaintext_key,omitempty"`
}

func toKeyResponse(k *store.APIKey) keyResponse {
	resp := keyResponse{
		ID:          k.ID,
		KeyPrefix:   k.KeyPrefix,
		Name:        k.Name,
		Environment: k.Environment,
		Status:      k.Status,
		CreatedAt:   k.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if k.LastUsedAt != nil {
		s := k.LastUsedAt.Format("2006-01-02T15:04:05Z")
		resp.LastUsedAt = &s
	}
	if k.ExpiresAt != nil {
		s := k.ExpiresAt.Format("2006-01-02T15:04:05Z")
		resp.ExpiresAt = &s
	}
	return resp
}

// CreateKey handles POST /api/v1/portal/apps/{app_id}/keys.
func (h *KeyHandler) CreateKey(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "X-User-ID header is required")
		return
	}

	appID := r.PathValue("app_id")
	if appID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "app_id is required")
		return
	}

	// Verify ownership
	app, err := h.appStore.GetByID(appID)
	if err != nil {
		if err == store.ErrAppNotFound {
			writeError(w, http.StatusNotFound, "not_found", "App not found")
			return
		}
		slog.Error("failed to get app", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get app")
		return
	}

	if app.OwnerUserID != userID {
		writeError(w, http.StatusForbidden, "forbidden", "You do not own this app")
		return
	}

	var req createKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	if req.Name == "" {
		req.Name = "Default"
	}

	// Generate key
	plaintext, hash, prefix, err := apikey.GenerateKey(app.Environment)
	if err != nil {
		slog.Error("failed to generate key", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to generate key")
		return
	}

	keyRecord := &store.APIKey{
		AppID:       appID,
		KeyHash:     hash,
		KeyPrefix:   prefix,
		Name:        req.Name,
		Environment: app.Environment,
	}

	if req.ExpiresInDays != nil && *req.ExpiresInDays > 0 {
		exp := time.Now().UTC().Add(time.Duration(*req.ExpiresInDays) * 24 * time.Hour)
		keyRecord.ExpiresAt = &exp
	}

	created, err := h.keyStore.Create(keyRecord)
	if err != nil {
		slog.Error("failed to store key", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create key")
		return
	}

	resp := toKeyResponse(created)
	resp.PlaintextKey = plaintext // Shown ONCE only

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// ListKeys handles GET /api/v1/portal/apps/{app_id}/keys.
func (h *KeyHandler) ListKeys(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "X-User-ID header is required")
		return
	}

	appID := r.PathValue("app_id")
	if appID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "app_id is required")
		return
	}

	// Verify ownership
	app, err := h.appStore.GetByID(appID)
	if err != nil {
		if err == store.ErrAppNotFound {
			writeError(w, http.StatusNotFound, "not_found", "App not found")
			return
		}
		slog.Error("failed to get app", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get app")
		return
	}

	if app.OwnerUserID != userID {
		writeError(w, http.StatusForbidden, "forbidden", "You do not own this app")
		return
	}

	keys, err := h.keyStore.ListByApp(appID)
	if err != nil {
		slog.Error("failed to list keys", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list keys")
		return
	}

	resp := make([]keyResponse, 0, len(keys))
	for _, k := range keys {
		resp = append(resp, toKeyResponse(k))
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"keys": resp})
}

// RevokeKey handles DELETE /api/v1/portal/apps/{app_id}/keys/{key_id}.
func (h *KeyHandler) RevokeKey(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "X-User-ID header is required")
		return
	}

	appID := r.PathValue("app_id")
	keyID := r.PathValue("key_id")

	// Verify ownership
	app, err := h.appStore.GetByID(appID)
	if err != nil {
		if err == store.ErrAppNotFound {
			writeError(w, http.StatusNotFound, "not_found", "App not found")
			return
		}
		slog.Error("failed to get app", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get app")
		return
	}

	if app.OwnerUserID != userID {
		writeError(w, http.StatusForbidden, "forbidden", "You do not own this app")
		return
	}

	err = h.keyStore.Revoke(keyID)
	if err != nil {
		if err == store.ErrKeyNotFound {
			writeError(w, http.StatusNotFound, "not_found", "Key not found")
			return
		}
		slog.Error("failed to revoke key", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to revoke key")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"id":         keyID,
		"status":     "REVOKED",
		"revoked_at": time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	})
}
