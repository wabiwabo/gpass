package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/garudapass/gpass/services/garudaportal/store"
)

// WebhookDispatcher dispatches a webhook delivery immediately.
type WebhookDispatcher interface {
	Deliver(sub *store.WebhookSubscription, eventType string, payload []byte) error
}

// ReplayHandler handles webhook delivery replay requests.
type ReplayHandler struct {
	deliveryStore store.DeliveryStore
	webhookStore  store.WebhookStore
	appStore      store.AppStore
	dispatcher    WebhookDispatcher
}

// NewReplayHandler creates a new replay handler.
func NewReplayHandler(
	deliveryStore store.DeliveryStore,
	webhookStore store.WebhookStore,
	appStore store.AppStore,
	dispatcher WebhookDispatcher,
) *ReplayHandler {
	return &ReplayHandler{
		deliveryStore: deliveryStore,
		webhookStore:  webhookStore,
		appStore:      appStore,
		dispatcher:    dispatcher,
	}
}

type deliveryResponse struct {
	ID               string  `json:"id"`
	SubscriptionID   string  `json:"subscription_id"`
	EventType        string  `json:"event_type"`
	Status           string  `json:"status"`
	Attempts         int     `json:"attempts"`
	LastResponseCode int     `json:"last_response_code"`
	LastResponseBody string  `json:"last_response_body,omitempty"`
	DeliveredAt      *string `json:"delivered_at,omitempty"`
	CreatedAt        string  `json:"created_at"`
}

func toDeliveryResponse(d *store.WebhookDelivery) deliveryResponse {
	resp := deliveryResponse{
		ID:               d.ID,
		SubscriptionID:   d.SubscriptionID,
		EventType:        d.EventType,
		Status:           d.Status,
		Attempts:         d.Attempts,
		LastResponseCode: d.LastResponseCode,
		LastResponseBody: d.LastResponseBody,
		CreatedAt:        d.CreatedAt.Format(time.RFC3339),
	}
	if d.DeliveredAt != nil {
		t := d.DeliveredAt.Format(time.RFC3339)
		resp.DeliveredAt = &t
	}
	return resp
}

// ReplayDelivery handles POST /api/v1/portal/apps/{app_id}/webhooks/deliveries/{delivery_id}/replay.
func (h *ReplayHandler) ReplayDelivery(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "X-User-ID header is required")
		return
	}

	appID := r.PathValue("app_id")
	deliveryID := r.PathValue("delivery_id")

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

	// Get the delivery
	delivery, err := h.deliveryStore.GetByID(deliveryID)
	if err != nil {
		if err == store.ErrDeliveryNotFound {
			writeError(w, http.StatusNotFound, "not_found", "Delivery not found")
			return
		}
		slog.Error("failed to get delivery", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get delivery")
		return
	}

	// Get the webhook subscription to verify it belongs to this app
	sub, err := h.webhookStore.GetByID(delivery.SubscriptionID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Webhook subscription not found")
		return
	}

	if sub.AppID != appID {
		writeError(w, http.StatusForbidden, "forbidden", "Delivery does not belong to this app")
		return
	}

	// Reset delivery for replay
	if err := h.deliveryStore.ResetForReplay(deliveryID); err != nil {
		slog.Error("failed to reset delivery for replay", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to reset delivery")
		return
	}

	// Dispatch immediately
	err = h.dispatcher.Deliver(sub, delivery.EventType, []byte(delivery.Payload))
	if err != nil {
		slog.Warn("replay delivery failed", "error", err, "delivery_id", deliveryID)
	}

	// Get updated delivery
	updated, err := h.deliveryStore.GetByID(deliveryID)
	if err != nil {
		slog.Error("failed to get updated delivery", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get delivery result")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(toDeliveryResponse(updated))
}

// ListDeliveries handles GET /api/v1/portal/apps/{app_id}/webhooks/deliveries.
func (h *ReplayHandler) ListDeliveries(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "X-User-ID header is required")
		return
	}

	appID := r.PathValue("app_id")

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

	status := r.URL.Query().Get("status")
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	// Get all webhook subscriptions for this app
	subs, err := h.webhookStore.ListByApp(appID)
	if err != nil {
		slog.Error("failed to list webhooks", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list webhooks")
		return
	}

	var allDeliveries []deliveryResponse
	for _, sub := range subs {
		deliveries, err := h.deliveryStore.ListBySubscription(sub.ID, status, limit)
		if err != nil {
			slog.Error("failed to list deliveries", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list deliveries")
			return
		}
		for _, d := range deliveries {
			allDeliveries = append(allDeliveries, toDeliveryResponse(d))
		}
	}

	if allDeliveries == nil {
		allDeliveries = []deliveryResponse{}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"deliveries": allDeliveries})
}
