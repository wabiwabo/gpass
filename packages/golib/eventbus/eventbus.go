// Package eventbus provides an in-process event bus for
// publish/subscribe messaging between components. Supports
// typed events, multiple subscribers, and async delivery.
package eventbus

import (
	"sync"
)

// Handler is a function that handles an event.
type Handler func(event interface{})

// Bus is an in-process event bus.
type Bus struct {
	mu          sync.RWMutex
	subscribers map[string][]Handler
}

// New creates an event bus.
func New() *Bus {
	return &Bus{subscribers: make(map[string][]Handler)}
}

// Subscribe registers a handler for an event topic.
func (b *Bus) Subscribe(topic string, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribers[topic] = append(b.subscribers[topic], handler)
}

// Publish sends an event to all subscribers of the topic.
// Handlers are called synchronously in registration order.
func (b *Bus) Publish(topic string, event interface{}) {
	b.mu.RLock()
	handlers := make([]Handler, len(b.subscribers[topic]))
	copy(handlers, b.subscribers[topic])
	b.mu.RUnlock()

	for _, h := range handlers {
		h(event)
	}
}

// PublishAsync sends an event to all subscribers asynchronously.
func (b *Bus) PublishAsync(topic string, event interface{}) {
	b.mu.RLock()
	handlers := make([]Handler, len(b.subscribers[topic]))
	copy(handlers, b.subscribers[topic])
	b.mu.RUnlock()

	for _, h := range handlers {
		go h(event)
	}
}

// HasSubscribers checks if a topic has any subscribers.
func (b *Bus) HasSubscribers(topic string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers[topic]) > 0
}

// Topics returns all topics that have subscribers.
func (b *Bus) Topics() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	topics := make([]string, 0, len(b.subscribers))
	for t, handlers := range b.subscribers {
		if len(handlers) > 0 {
			topics = append(topics, t)
		}
	}
	return topics
}

// SubscriberCount returns the number of subscribers for a topic.
func (b *Bus) SubscriberCount(topic string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers[topic])
}

// Clear removes all subscribers.
func (b *Bus) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribers = make(map[string][]Handler)
}
