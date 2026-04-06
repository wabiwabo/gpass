package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/garudaportal/store"
)

func TestWebhookHandler_Subscribe(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	webhookStore := store.NewInMemoryWebhookStore()
	app, _ := appStore.Create(&store.App{OwnerUserID: "user-1", Name: "My App", Environment: "sandbox", Tier: "free", DailyLimit: 100})

	h := NewWebhookHandler(appStore, webhookStore)

	body := `{"url":"https://example.com/webhook","events":["identity.verified","document.signed"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+app.ID+"/webhooks", bytes.NewBufferString(body))
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("app_id", app.ID)
	w := httptest.NewRecorder()

	h.Subscribe(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp webhookResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.URL != "https://example.com/webhook" {
		t.Errorf("expected URL https://example.com/webhook, got %s", resp.URL)
	}
	if resp.Secret == "" {
		t.Error("expected secret to be returned")
	}
	if len(resp.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(resp.Events))
	}
}

func TestWebhookHandler_Subscribe_NonHTTPS(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	webhookStore := store.NewInMemoryWebhookStore()
	app, _ := appStore.Create(&store.App{OwnerUserID: "user-1", Name: "My App", Environment: "sandbox", Tier: "free", DailyLimit: 100})

	h := NewWebhookHandler(appStore, webhookStore)

	body := `{"url":"http://example.com/webhook","events":["identity.verified"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+app.ID+"/webhooks", bytes.NewBufferString(body))
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("app_id", app.ID)
	w := httptest.NewRecorder()

	h.Subscribe(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWebhookHandler_Subscribe_EmptyEvents(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	webhookStore := store.NewInMemoryWebhookStore()
	app, _ := appStore.Create(&store.App{OwnerUserID: "user-1", Name: "My App", Environment: "sandbox", Tier: "free", DailyLimit: 100})

	h := NewWebhookHandler(appStore, webhookStore)

	body := `{"url":"https://example.com/webhook","events":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+app.ID+"/webhooks", bytes.NewBufferString(body))
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("app_id", app.ID)
	w := httptest.NewRecorder()

	h.Subscribe(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestWebhookHandler_ListWebhooks(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	webhookStore := store.NewInMemoryWebhookStore()
	app, _ := appStore.Create(&store.App{OwnerUserID: "user-1", Name: "My App", Environment: "sandbox", Tier: "free", DailyLimit: 100})

	webhookStore.Create(&store.WebhookSubscription{AppID: app.ID, URL: "https://a.com/hook", Events: []string{"e1"}, Secret: "s1"})
	webhookStore.Create(&store.WebhookSubscription{AppID: app.ID, URL: "https://b.com/hook", Events: []string{"e2"}, Secret: "s2"})

	h := NewWebhookHandler(appStore, webhookStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/apps/"+app.ID+"/webhooks", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("app_id", app.ID)
	w := httptest.NewRecorder()

	h.ListWebhooks(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Webhooks []webhookResponse `json:"webhooks"`
	}
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Webhooks) != 2 {
		t.Errorf("expected 2 webhooks, got %d", len(resp.Webhooks))
	}
}

func TestWebhookHandler_DeleteWebhook(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	webhookStore := store.NewInMemoryWebhookStore()
	app, _ := appStore.Create(&store.App{OwnerUserID: "user-1", Name: "My App", Environment: "sandbox", Tier: "free", DailyLimit: 100})
	sub, _ := webhookStore.Create(&store.WebhookSubscription{AppID: app.ID, URL: "https://a.com/hook", Events: []string{"e1"}, Secret: "s1"})

	h := NewWebhookHandler(appStore, webhookStore)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/portal/apps/"+app.ID+"/webhooks/"+sub.ID, nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("app_id", app.ID)
	req.SetPathValue("webhook_id", sub.ID)
	w := httptest.NewRecorder()

	h.DeleteWebhook(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "DISABLED" {
		t.Errorf("expected DISABLED, got %v", resp["status"])
	}
}
