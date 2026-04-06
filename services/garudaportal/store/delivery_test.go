package store

import (
	"testing"
	"time"
)

func TestDeliveryStore_CreateAndGet(t *testing.T) {
	s := NewInMemoryDeliveryStore()

	d, err := s.Create(&WebhookDelivery{
		SubscriptionID: "sub-1",
		EventType:      "identity.verified",
		Payload:        `{"user_id":"123"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if d.ID == "" {
		t.Error("expected ID to be set")
	}
	if d.Status != "PENDING" {
		t.Errorf("expected status PENDING, got %s", d.Status)
	}

	got, err := s.GetByID(d.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.EventType != "identity.verified" {
		t.Errorf("unexpected event type: %s", got.EventType)
	}
}

func TestDeliveryStore_GetByID_NotFound(t *testing.T) {
	s := NewInMemoryDeliveryStore()

	_, err := s.GetByID("nonexistent")
	if err != ErrDeliveryNotFound {
		t.Fatalf("expected ErrDeliveryNotFound, got %v", err)
	}
}

func TestDeliveryStore_UpdateStatus(t *testing.T) {
	s := NewInMemoryDeliveryStore()

	d, _ := s.Create(&WebhookDelivery{
		SubscriptionID: "sub-1",
		EventType:      "identity.verified",
		Payload:        `{"user_id":"123"}`,
	})

	err := s.UpdateStatus(d.ID, "DELIVERED", 200, `{"ok":true}`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := s.GetByID(d.ID)
	if got.Status != "DELIVERED" {
		t.Errorf("expected DELIVERED, got %s", got.Status)
	}
	if got.LastResponseCode != 200 {
		t.Errorf("expected response code 200, got %d", got.LastResponseCode)
	}
	if got.Attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", got.Attempts)
	}
	if got.DeliveredAt == nil {
		t.Error("expected DeliveredAt to be set")
	}
}

func TestDeliveryStore_UpdateStatus_NotFound(t *testing.T) {
	s := NewInMemoryDeliveryStore()

	err := s.UpdateStatus("nonexistent", "DELIVERED", 200, "", nil)
	if err != ErrDeliveryNotFound {
		t.Fatalf("expected ErrDeliveryNotFound, got %v", err)
	}
}

func TestDeliveryStore_ListPending(t *testing.T) {
	s := NewInMemoryDeliveryStore()

	// Pending with no retry (immediate)
	s.Create(&WebhookDelivery{
		SubscriptionID: "sub-1",
		EventType:      "identity.verified",
		Payload:        `{"id":"1"}`,
	})

	// Pending with past retry time
	pastTime := time.Now().UTC().Add(-1 * time.Minute)
	d2, _ := s.Create(&WebhookDelivery{
		SubscriptionID: "sub-2",
		EventType:      "document.signed",
		Payload:        `{"id":"2"}`,
	})
	s.UpdateStatus(d2.ID, "PENDING", 500, "error", &pastTime)

	// Pending with future retry time (should NOT be included)
	futureTime := time.Now().UTC().Add(1 * time.Hour)
	d3, _ := s.Create(&WebhookDelivery{
		SubscriptionID: "sub-3",
		EventType:      "document.signed",
		Payload:        `{"id":"3"}`,
	})
	s.UpdateStatus(d3.ID, "PENDING", 500, "error", &futureTime)

	// Delivered (should NOT be included)
	d4, _ := s.Create(&WebhookDelivery{
		SubscriptionID: "sub-4",
		EventType:      "identity.verified",
		Payload:        `{"id":"4"}`,
	})
	s.UpdateStatus(d4.ID, "DELIVERED", 200, "ok", nil)

	pending, err := s.ListPending()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pending) != 2 {
		t.Errorf("expected 2 pending deliveries, got %d", len(pending))
	}
}
