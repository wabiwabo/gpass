package events

import (
	"testing"
	"time"
)

func TestMemoryBus_PublishAndVerifyStored(t *testing.T) {
	bus := NewMemoryBus()

	event := Event{
		Type:   "identity.verified",
		Source: "identity",
	}

	if err := bus.Publish("identity.events", event); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	published := bus.Published()
	if len(published) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(published))
	}

	if published[0].Type != "identity.verified" {
		t.Errorf("expected type %q, got %q", "identity.verified", published[0].Type)
	}
}

func TestMemoryBus_SubscribeAndReceive(t *testing.T) {
	bus := NewMemoryBus()

	var received []Event
	if err := bus.Subscribe("test.topic", func(e Event) error {
		received = append(received, e)
		return nil
	}); err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	event := Event{Type: "test.event", Source: "test"}
	if err := bus.Publish("test.topic", event); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	if len(received) != 1 {
		t.Fatalf("expected 1 received event, got %d", len(received))
	}
	if received[0].Type != "test.event" {
		t.Errorf("expected type %q, got %q", "test.event", received[0].Type)
	}
}

func TestMemoryBus_MultipleSubscribers(t *testing.T) {
	bus := NewMemoryBus()

	var received1, received2 []Event

	bus.Subscribe("test.topic", func(e Event) error {
		received1 = append(received1, e)
		return nil
	})
	bus.Subscribe("test.topic", func(e Event) error {
		received2 = append(received2, e)
		return nil
	})

	event := Event{Type: "test.event", Source: "test"}
	if err := bus.Publish("test.topic", event); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	if len(received1) != 1 {
		t.Errorf("subscriber 1: expected 1 event, got %d", len(received1))
	}
	if len(received2) != 1 {
		t.Errorf("subscriber 2: expected 1 event, got %d", len(received2))
	}
}

func TestMemoryBus_PublishNoSubscribers(t *testing.T) {
	bus := NewMemoryBus()

	event := Event{Type: "test.event", Source: "test"}
	if err := bus.Publish("test.topic", event); err != nil {
		t.Fatalf("Publish() to topic with no subscribers should not error, got %v", err)
	}

	if len(bus.Published()) != 1 {
		t.Errorf("event should still be stored even with no subscribers")
	}
}

func TestMemoryBus_GeneratesIDAndTimestamp(t *testing.T) {
	bus := NewMemoryBus()

	event := Event{Type: "test.event", Source: "test"}
	before := time.Now().UTC()

	if err := bus.Publish("test.topic", event); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	after := time.Now().UTC()
	published := bus.Published()

	if published[0].ID == "" {
		t.Error("expected non-empty ID")
	}
	if published[0].Timestamp.Before(before) || published[0].Timestamp.After(after) {
		t.Errorf("timestamp %v not between %v and %v", published[0].Timestamp, before, after)
	}
}

func TestMemoryBus_CloseIdempotent(t *testing.T) {
	bus := NewMemoryBus()

	if err := bus.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := bus.Close(); err != nil {
		t.Fatalf("Close() second call error = %v", err)
	}
}

func TestMemoryBus_ImplementsInterfaces(t *testing.T) {
	bus := NewMemoryBus()
	var _ Publisher = bus
	var _ Subscriber = bus
}
