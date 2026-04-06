package events

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"time"
)

// LogPublisher is a Publisher that writes events to slog for dev/test use.
type LogPublisher struct{}

// NewLogPublisher creates a new LogPublisher.
func NewLogPublisher() *LogPublisher {
	return &LogPublisher{}
}

// Publish logs the event using slog and populates ID/Timestamp if empty.
func (p *LogPublisher) Publish(topic string, event Event) error {
	if event.ID == "" {
		id, err := generateID()
		if err != nil {
			return err
		}
		event.ID = id
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	slog.Info("event published",
		"topic", topic,
		"event_id", event.ID,
		"event_type", event.Type,
		"source", event.Source,
		"subject", event.Subject,
		"timestamp", event.Timestamp,
	)
	return nil
}

// Close is a no-op for LogPublisher.
func (p *LogPublisher) Close() error {
	return nil
}

func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
