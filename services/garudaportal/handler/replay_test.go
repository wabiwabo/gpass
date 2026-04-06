package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/garudaportal/store"
)

// mockDispatcher implements WebhookDispatcher for testing.
type mockDispatcher struct {
	delivered bool
	lastSub   *store.WebhookSubscription
	lastEvent string
	err       error
}

func (m *mockDispatcher) Deliver(sub *store.WebhookSubscription, eventType string, payload []byte) error {
	m.delivered = true
	m.lastSub = sub
	m.lastEvent = eventType
	return m.err
}

func setupReplayTest(t *testing.T) (*store.InMemoryAppStore, *store.InMemoryWebhookStore, *store.InMemoryDeliveryStore, *store.App, *store.WebhookSubscription) {
	t.Helper()
	appStore := store.NewInMemoryAppStore()
	webhookStore := store.NewInMemoryWebhookStore()
	deliveryStore := store.NewInMemoryDeliveryStore()

	app, err := appStore.Create(&store.App{OwnerUserID: "user-1", Name: "Test App", Environment: "sandbox", Tier: "free", DailyLimit: 100})
	if err != nil {
		t.Fatal(err)
	}

	sub, err := webhookStore.Create(&store.WebhookSubscription{
		AppID:  app.ID,
		URL:    "https://example.com/webhook",
		Events: []string{"identity.verified"},
		Secret: "whsec_test",
	})
	if err != nil {
		t.Fatal(err)
	}

	return appStore, webhookStore, deliveryStore, app, sub
}

func TestReplayHandler_ReplayDelivery_Success(t *testing.T) {
	appStore, webhookStore, deliveryStore, app, sub := setupReplayTest(t)

	// Create a failed delivery
	delivery, _ := deliveryStore.Create(&store.WebhookDelivery{
		SubscriptionID: sub.ID,
		EventType:      "identity.verified",
		Payload:        `{"user":"test"}`,
		Status:         "FAILED",
	})
	deliveryStore.UpdateStatus(delivery.ID, "FAILED", 500, "Internal Server Error", nil)

	disp := &mockDispatcher{}
	h := NewReplayHandler(deliveryStore, webhookStore, appStore, disp)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+app.ID+"/webhooks/deliveries/"+delivery.ID+"/replay", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("app_id", app.ID)
	req.SetPathValue("delivery_id", delivery.ID)
	w := httptest.NewRecorder()

	h.ReplayDelivery(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if !disp.delivered {
		t.Error("expected dispatcher to be called")
	}

	var resp deliveryResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.ID != delivery.ID {
		t.Errorf("expected delivery ID %s, got %s", delivery.ID, resp.ID)
	}

	// Verify the delivery was reset before dispatch
	if disp.lastEvent != "identity.verified" {
		t.Errorf("expected event identity.verified, got %s", disp.lastEvent)
	}
}

func TestReplayHandler_ReplayDelivery_NotFound(t *testing.T) {
	appStore, webhookStore, deliveryStore, app, _ := setupReplayTest(t)

	disp := &mockDispatcher{}
	h := NewReplayHandler(deliveryStore, webhookStore, appStore, disp)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+app.ID+"/webhooks/deliveries/nonexistent/replay", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("app_id", app.ID)
	req.SetPathValue("delivery_id", "nonexistent")
	w := httptest.NewRecorder()

	h.ReplayDelivery(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReplayHandler_ReplayDelivery_NotOwner(t *testing.T) {
	appStore, webhookStore, deliveryStore, app, sub := setupReplayTest(t)

	delivery, _ := deliveryStore.Create(&store.WebhookDelivery{
		SubscriptionID: sub.ID,
		EventType:      "identity.verified",
		Payload:        `{"user":"test"}`,
		Status:         "FAILED",
	})

	disp := &mockDispatcher{}
	h := NewReplayHandler(deliveryStore, webhookStore, appStore, disp)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+app.ID+"/webhooks/deliveries/"+delivery.ID+"/replay", nil)
	req.Header.Set("X-User-ID", "user-other")
	req.SetPathValue("app_id", app.ID)
	req.SetPathValue("delivery_id", delivery.ID)
	w := httptest.NewRecorder()

	h.ReplayDelivery(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}

	if disp.delivered {
		t.Error("dispatcher should not have been called")
	}
}

func TestReplayHandler_ListDeliveries_WithStatusFilter(t *testing.T) {
	appStore, webhookStore, deliveryStore, app, sub := setupReplayTest(t)

	// Create deliveries with different statuses
	d1, _ := deliveryStore.Create(&store.WebhookDelivery{
		SubscriptionID: sub.ID,
		EventType:      "identity.verified",
		Payload:        `{"user":"1"}`,
		Status:         "FAILED",
	})
	deliveryStore.UpdateStatus(d1.ID, "FAILED", 500, "error", nil)

	deliveryStore.Create(&store.WebhookDelivery{
		SubscriptionID: sub.ID,
		EventType:      "identity.verified",
		Payload:        `{"user":"2"}`,
		Status:         "DELIVERED",
	})

	disp := &mockDispatcher{}
	h := NewReplayHandler(deliveryStore, webhookStore, appStore, disp)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/apps/"+app.ID+"/webhooks/deliveries?status=FAILED&limit=20", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("app_id", app.ID)
	w := httptest.NewRecorder()

	h.ListDeliveries(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Deliveries []deliveryResponse `json:"deliveries"`
	}
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Deliveries) != 1 {
		t.Errorf("expected 1 delivery, got %d", len(resp.Deliveries))
	}

	if len(resp.Deliveries) > 0 && resp.Deliveries[0].Status != "FAILED" {
		t.Errorf("expected status FAILED, got %s", resp.Deliveries[0].Status)
	}
}

func TestReplayHandler_ListDeliveries_Empty(t *testing.T) {
	appStore, webhookStore, deliveryStore, app, _ := setupReplayTest(t)

	disp := &mockDispatcher{}
	h := NewReplayHandler(deliveryStore, webhookStore, appStore, disp)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/apps/"+app.ID+"/webhooks/deliveries", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("app_id", app.ID)
	w := httptest.NewRecorder()

	h.ListDeliveries(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Deliveries []deliveryResponse `json:"deliveries"`
	}
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Deliveries) != 0 {
		t.Errorf("expected 0 deliveries, got %d", len(resp.Deliveries))
	}
}
