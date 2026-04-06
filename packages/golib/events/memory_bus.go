package events

import (
	"sync"
	"time"
)

// MemoryBus is an in-memory event bus for testing. It implements both Publisher and Subscriber.
type MemoryBus struct {
	handlers  map[string][]func(Event) error
	published []Event
	mu        sync.RWMutex
}

// NewMemoryBus creates a new MemoryBus.
func NewMemoryBus() *MemoryBus {
	return &MemoryBus{
		handlers: make(map[string][]func(Event) error),
	}
}

// Publish stores the event and dispatches it to all subscribers for the topic.
// It populates ID and Timestamp if they are empty.
func (b *MemoryBus) Publish(topic string, event Event) error {
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

	b.mu.Lock()
	b.published = append(b.published, event)
	handlers := make([]func(Event) error, len(b.handlers[topic]))
	copy(handlers, b.handlers[topic])
	b.mu.Unlock()

	for _, h := range handlers {
		if err := h(event); err != nil {
			return err
		}
	}
	return nil
}

// Subscribe registers a handler for the given topic.
func (b *MemoryBus) Subscribe(topic string, handler func(Event) error) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[topic] = append(b.handlers[topic], handler)
	return nil
}

// Published returns all events that have been published.
func (b *MemoryBus) Published() []Event {
	b.mu.RLock()
	defer b.mu.RUnlock()
	result := make([]Event, len(b.published))
	copy(result, b.published)
	return result
}

// Close is a no-op for MemoryBus.
func (b *MemoryBus) Close() error {
	return nil
}
