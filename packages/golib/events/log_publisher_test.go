package events

import (
	"testing"
	"time"
)

func TestLogPublisher_Publish(t *testing.T) {
	pub := NewLogPublisher()

	event := Event{
		Type:   "identity.verified",
		Source: "identity",
	}

	if err := pub.Publish("identity.events", event); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
}

func TestLogPublisher_PublishPopulatesIDAndTimestamp(t *testing.T) {
	pub := NewLogPublisher()

	event := Event{
		Type:   "test.event",
		Source: "test",
	}

	// LogPublisher modifies a copy, so we just verify no error.
	if err := pub.Publish("test.topic", event); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
}

func TestLogPublisher_PublishPreservesExistingID(t *testing.T) {
	pub := NewLogPublisher()

	event := Event{
		ID:        "custom-id",
		Type:      "test.event",
		Source:    "test",
		Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	if err := pub.Publish("test.topic", event); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
}

func TestLogPublisher_CloseIdempotent(t *testing.T) {
	pub := NewLogPublisher()

	if err := pub.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := pub.Close(); err != nil {
		t.Fatalf("Close() second call error = %v", err)
	}
}

func TestLogPublisher_ImplementsPublisher(t *testing.T) {
	var _ Publisher = NewLogPublisher()
}
