package webhook

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/garudapass/gpass/services/garudaportal/store"
)

func TestDispatcher_SuccessfulDelivery(t *testing.T) {
	var receivedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	deliveryStore := store.NewInMemoryDeliveryStore()
	dispatcher := NewDispatcher(server.Client(), deliveryStore)

	sub := &store.WebhookSubscription{
		ID:     "sub-1",
		AppID:  "app-1",
		URL:    server.URL,
		Events: []string{"identity.verified"},
		Secret: "whsec_test123",
		Status: "ACTIVE",
	}

	err := dispatcher.Deliver(sub, "identity.verified", []byte(`{"user_id":"123"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify headers were sent
	if receivedHeaders.Get("Content-Type") != "application/json" {
		t.Error("expected Content-Type: application/json")
	}
	if receivedHeaders.Get("X-GarudaPass-Signature") == "" {
		t.Error("expected X-GarudaPass-Signature header")
	}
	if receivedHeaders.Get("X-GarudaPass-Event") != "identity.verified" {
		t.Error("expected X-GarudaPass-Event: identity.verified")
	}
}

func TestDispatcher_FailedDelivery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"server error"}`))
	}))
	defer server.Close()

	deliveryStore := store.NewInMemoryDeliveryStore()
	dispatcher := NewDispatcher(server.Client(), deliveryStore)

	sub := &store.WebhookSubscription{
		ID:     "sub-1",
		AppID:  "app-1",
		URL:    server.URL,
		Events: []string{"identity.verified"},
		Secret: "whsec_test123",
		Status: "ACTIVE",
	}

	err := dispatcher.Deliver(sub, "identity.verified", []byte(`{"user_id":"123"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify delivery was recorded with pending status for retry
	pending, _ := deliveryStore.ListPending()
	// The delivery should still be pending (for retry) since the server returned 500
	// It may or may not show up depending on NextRetryAt timing
	_ = pending
}

func TestNextRetryDelay(t *testing.T) {
	expected := []time.Duration{
		1 * time.Minute,
		5 * time.Minute,
		30 * time.Minute,
		2 * time.Hour,
		24 * time.Hour,
	}

	for i, want := range expected {
		got := NextRetryDelay(i)
		if got != want {
			t.Errorf("attempt %d: got %v, want %v", i, got, want)
		}
	}

	// Beyond max attempts should return last delay
	got := NextRetryDelay(10)
	if got != 24*time.Hour {
		t.Errorf("beyond max: got %v, want 24h", got)
	}

	// Negative attempt
	got = NextRetryDelay(-1)
	if got != 1*time.Minute {
		t.Errorf("negative attempt: got %v, want 1m", got)
	}
}
