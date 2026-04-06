package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/garudapass/gpass/services/garudaportal/apikey"
	"github.com/garudapass/gpass/services/garudaportal/store"
)

// RotationHandler handles API key rotation.
type RotationHandler struct {
	appStore store.AppStore
	keyStore store.KeyStore
}

// NewRotationHandler creates a new rotation handler.
func NewRotationHandler(appStore store.AppStore, keyStore store.KeyStore) *RotationHandler {
	return &RotationHandler{
		appStore: appStore,
		keyStore: keyStore,
	}
}

type rotationNewKeyResponse struct {
	ID           string `json:"id"`
	PlaintextKey string `json:"plaintext_key"`
	KeyPrefix    string `json:"key_prefix"`
	Environment  string `json:"environment"`
	Status       string `json:"status"`
}

type rotationOldKeyResponse struct {
	ID        string `json:"id"`
	KeyPrefix string `json:"key_prefix"`
	Status    string `json:"status"`
	Note      string `json:"note"`
}

type rotationResponse struct {
	NewKey               rotationNewKeyResponse `json:"new_key"`
	OldKey               rotationOldKeyResponse `json:"old_key"`
	RotationInstructions []string               `json:"rotation_instructions"`
}

// RotateKey handles POST /api/v1/portal/apps/{app_id}/keys/{key_id}/rotate.
func (h *RotationHandler) RotateKey(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "X-User-ID header is required")
		return
	}

	appID := r.PathValue("app_id")
	keyID := r.PathValue("key_id")

	// Verify app ownership
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

	// Verify old key exists and is active
	oldKey, err := h.keyStore.GetByID(keyID)
	if err != nil {
		if err == store.ErrKeyNotFound {
			writeError(w, http.StatusNotFound, "not_found", "Key not found")
			return
		}
		slog.Error("failed to get key", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get key")
		return
	}

	if oldKey.Status == "REVOKED" {
		writeError(w, http.StatusConflict, "already_revoked", "Cannot rotate a revoked key")
		return
	}

	// Generate new key with same environment
	plaintext, hash, prefix, err := apikey.GenerateKey(oldKey.Environment)
	if err != nil {
		slog.Error("failed to generate key", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to generate key")
		return
	}

	newKeyRecord := &store.APIKey{
		AppID:       appID,
		KeyHash:     hash,
		KeyPrefix:   prefix,
		Name:        oldKey.Name + " (rotated)",
		Environment: oldKey.Environment,
	}

	created, err := h.keyStore.Create(newKeyRecord)
	if err != nil {
		slog.Error("failed to store rotated key", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create rotated key")
		return
	}

	resp := rotationResponse{
		NewKey: rotationNewKeyResponse{
			ID:           created.ID,
			PlaintextKey: plaintext,
			KeyPrefix:    created.KeyPrefix,
			Environment:  created.Environment,
			Status:       created.Status,
		},
		OldKey: rotationOldKeyResponse{
			ID:        oldKey.ID,
			KeyPrefix: oldKey.KeyPrefix,
			Status:    oldKey.Status,
			Note:      "Revoke after migration",
		},
		RotationInstructions: []string{
			"1. Update your application with the new API key",
			"2. Deploy and verify the new key works",
			"3. Revoke the old key via DELETE /keys/{old_key_id}",
		},
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}
