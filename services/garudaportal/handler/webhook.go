package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/garudapass/gpass/services/garudaportal/store"
)

// WebhookHandler handles webhook subscription endpoints.
type WebhookHandler struct {
	appStore     store.AppStore
	webhookStore store.WebhookStore
}

// NewWebhookHandler creates a new webhook handler.
func NewWebhookHandler(appStore store.AppStore, webhookStore store.WebhookStore) *WebhookHandler {
	return &WebhookHandler{
		appStore:     appStore,
		webhookStore: webhookStore,
	}
}

type subscribeRequest struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
}

type webhookResponse struct {
	ID        string   `json:"id"`
	URL       string   `json:"url"`
	Events    []string `json:"events"`
	Status    string   `json:"status"`
	Secret    string   `json:"secret,omitempty"`
	CreatedAt string   `json:"created_at"`
}

func toWebhookResponse(w *store.WebhookSubscription, includeSecret bool) webhookResponse {
	resp := webhookResponse{
		ID:        w.ID,
		URL:       w.URL,
		Events:    w.Events,
		Status:    w.Status,
		CreatedAt: w.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if includeSecret {
		resp.Secret = w.Secret
	}
	return resp
}

// Subscribe handles POST /api/v1/portal/apps/{app_id}/webhooks.
func (h *WebhookHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
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

	var req subscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "url is required")
		return
	}

	if len(req.Events) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "events must not be empty")
		return
	}

	// Generate webhook secret
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		slog.Error("failed to generate webhook secret", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to generate secret")
		return
	}
	secret := "whsec_" + hex.EncodeToString(secretBytes)

	sub, err := h.webhookStore.Create(&store.WebhookSubscription{
		AppID:  appID,
		URL:    req.URL,
		Events: req.Events,
		Secret: secret,
	})
	if err != nil {
		if err == store.ErrWebhookURLScheme {
			writeError(w, http.StatusBadRequest, "invalid_request", "Webhook URL must use https (except localhost for development)")
			return
		}
		slog.Error("failed to create webhook", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create webhook")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toWebhookResponse(sub, true))
}

// ListWebhooks handles GET /api/v1/portal/apps/{app_id}/webhooks.
func (h *WebhookHandler) ListWebhooks(w http.ResponseWriter, r *http.Request) {
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

	subs, err := h.webhookStore.ListByApp(appID)
	if err != nil {
		slog.Error("failed to list webhooks", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list webhooks")
		return
	}

	resp := make([]webhookResponse, 0, len(subs))
	for _, s := range subs {
		resp = append(resp, toWebhookResponse(s, false))
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"webhooks": resp})
}

// DeleteWebhook handles DELETE /api/v1/portal/apps/{app_id}/webhooks/{webhook_id}.
func (h *WebhookHandler) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "X-User-ID header is required")
		return
	}

	appID := r.PathValue("app_id")
	webhookID := r.PathValue("webhook_id")

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

	err = h.webhookStore.Disable(webhookID)
	if err != nil {
		if err == store.ErrWebhookNotFound {
			writeError(w, http.StatusNotFound, "not_found", "Webhook not found")
			return
		}
		slog.Error("failed to disable webhook", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to delete webhook")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"id":     webhookID,
		"status": "DISABLED",
	})
}
