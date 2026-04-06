package events

import "time"

// Event represents a domain event published to Kafka.
type Event struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`    // e.g. "identity.verified", "document.signed"
	Source    string            `json:"source"`  // service name
	Subject   string            `json:"subject"` // resource ID
	Data      interface{}       `json:"data"`
	Metadata  map[string]string `json:"metadata"`
	Timestamp time.Time         `json:"timestamp"`
}

// Publisher publishes events to topics.
type Publisher interface {
	Publish(topic string, event Event) error
	Close() error
}

// Subscriber consumes events from topics.
type Subscriber interface {
	Subscribe(topic string, handler func(Event) error) error
	Close() error
}
