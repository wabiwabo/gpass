package store

import (
	"testing"
)

func TestWebhookStore_Create(t *testing.T) {
	s := NewInMemoryWebhookStore()

	sub, err := s.Create(&WebhookSubscription{
		AppID:  "app-1",
		URL:    "https://example.com/webhook",
		Events: []string{"identity.verified", "document.signed"},
		Secret: "whsec_test123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sub.ID == "" {
		t.Error("expected ID to be set")
	}
	if sub.Status != "ACTIVE" {
		t.Errorf("expected status ACTIVE, got %s", sub.Status)
	}
}

func TestWebhookStore_Create_InvalidURL(t *testing.T) {
	s := NewInMemoryWebhookStore()

	_, err := s.Create(&WebhookSubscription{
		AppID:  "app-1",
		URL:    "http://example.com/webhook",
		Events: []string{"identity.verified"},
		Secret: "whsec_test123",
	})
	if err != ErrWebhookURLScheme {
		t.Fatalf("expected ErrWebhookURLScheme, got %v", err)
	}
}

func TestWebhookStore_Create_LocalhostAllowed(t *testing.T) {
	s := NewInMemoryWebhookStore()

	_, err := s.Create(&WebhookSubscription{
		AppID:  "app-1",
		URL:    "http://localhost:8080/webhook",
		Events: []string{"identity.verified"},
		Secret: "whsec_test123",
	})
	if err != nil {
		t.Fatalf("localhost should be allowed: %v", err)
	}
}

func TestWebhookStore_GetByID(t *testing.T) {
	s := NewInMemoryWebhookStore()

	sub, _ := s.Create(&WebhookSubscription{
		AppID:  "app-1",
		URL:    "https://example.com/webhook",
		Events: []string{"identity.verified"},
		Secret: "whsec_test123",
	})

	got, err := s.GetByID(sub.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.URL != "https://example.com/webhook" {
		t.Errorf("unexpected URL: %s", got.URL)
	}
}

func TestWebhookStore_GetByID_NotFound(t *testing.T) {
	s := NewInMemoryWebhookStore()

	_, err := s.GetByID("nonexistent")
	if err != ErrWebhookNotFound {
		t.Fatalf("expected ErrWebhookNotFound, got %v", err)
	}
}

func TestWebhookStore_ListByApp(t *testing.T) {
	s := NewInMemoryWebhookStore()

	s.Create(&WebhookSubscription{AppID: "app-1", URL: "https://a.com/hook", Events: []string{"e1"}, Secret: "s1"})
	s.Create(&WebhookSubscription{AppID: "app-1", URL: "https://b.com/hook", Events: []string{"e2"}, Secret: "s2"})
	s.Create(&WebhookSubscription{AppID: "app-2", URL: "https://c.com/hook", Events: []string{"e1"}, Secret: "s3"})

	subs, err := s.ListByApp("app-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(subs) != 2 {
		t.Errorf("expected 2 subs, got %d", len(subs))
	}
}

func TestWebhookStore_ListByEvent(t *testing.T) {
	s := NewInMemoryWebhookStore()

	s.Create(&WebhookSubscription{AppID: "app-1", URL: "https://a.com/hook", Events: []string{"identity.verified", "document.signed"}, Secret: "s1"})
	s.Create(&WebhookSubscription{AppID: "app-2", URL: "https://b.com/hook", Events: []string{"identity.verified"}, Secret: "s2"})
	s.Create(&WebhookSubscription{AppID: "app-3", URL: "https://c.com/hook", Events: []string{"document.signed"}, Secret: "s3"})

	subs, err := s.ListByEvent("identity.verified")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(subs) != 2 {
		t.Errorf("expected 2 subs matching identity.verified, got %d", len(subs))
	}
}

func TestWebhookStore_ListByEvent_DisabledExcluded(t *testing.T) {
	s := NewInMemoryWebhookStore()

	sub, _ := s.Create(&WebhookSubscription{AppID: "app-1", URL: "https://a.com/hook", Events: []string{"identity.verified"}, Secret: "s1"})
	s.Disable(sub.ID)

	subs, err := s.ListByEvent("identity.verified")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(subs) != 0 {
		t.Errorf("expected 0 subs (disabled excluded), got %d", len(subs))
	}
}

func TestWebhookStore_Disable(t *testing.T) {
	s := NewInMemoryWebhookStore()

	sub, _ := s.Create(&WebhookSubscription{
		AppID:  "app-1",
		URL:    "https://example.com/hook",
		Events: []string{"identity.verified"},
		Secret: "s1",
	})

	err := s.Disable(sub.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := s.GetByID(sub.ID)
	if got.Status != "DISABLED" {
		t.Errorf("expected DISABLED, got %s", got.Status)
	}
}

func TestWebhookStore_Disable_NotFound(t *testing.T) {
	s := NewInMemoryWebhookStore()

	err := s.Disable("nonexistent")
	if err != ErrWebhookNotFound {
		t.Fatalf("expected ErrWebhookNotFound, got %v", err)
	}
}
