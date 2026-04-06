// Package safemap provides a type-safe concurrent map using generics.
// It wraps sync.RWMutex for fine-grained read/write locking and supports
// expiration, iteration, and bulk operations.
package safemap

import (
	"sync"
	"time"
)

// Map is a type-safe concurrent map.
type Map[K comparable, V any] struct {
	mu    sync.RWMutex
	items map[K]entry[V]
}

type entry[V any] struct {
	value     V
	expiresAt time.Time
}

// New creates a new concurrent map.
func New[K comparable, V any]() *Map[K, V] {
	return &Map[K, V]{
		items: make(map[K]entry[V]),
	}
}

// Set stores a key-value pair with no expiration.
func (m *Map[K, V]) Set(key K, value V) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[key] = entry[V]{value: value}
}

// SetWithTTL stores a key-value pair with a time-to-live.
func (m *Map[K, V]) SetWithTTL(key K, value V, ttl time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[key] = entry[V]{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
}

// Get retrieves a value. Returns the value and whether it was found.
func (m *Map[K, V]) Get(key K) (V, bool) {
	m.mu.RLock()
	e, ok := m.items[key]
	m.mu.RUnlock()

	if !ok {
		var zero V
		return zero, false
	}

	if !e.expiresAt.IsZero() && time.Now().After(e.expiresAt) {
		m.Delete(key)
		var zero V
		return zero, false
	}

	return e.value, true
}

// GetOrSet returns the existing value for key, or stores and returns the new value.
func (m *Map[K, V]) GetOrSet(key K, value V) (V, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if e, ok := m.items[key]; ok {
		if e.expiresAt.IsZero() || time.Now().Before(e.expiresAt) {
			return e.value, true // Existed.
		}
	}

	m.items[key] = entry[V]{value: value}
	return value, false // New.
}

// Delete removes a key.
func (m *Map[K, V]) Delete(key K) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, key)
}

// Has checks if a key exists (and is not expired).
func (m *Map[K, V]) Has(key K) bool {
	_, ok := m.Get(key)
	return ok
}

// Len returns the number of items (including expired).
func (m *Map[K, V]) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.items)
}

// Keys returns all non-expired keys.
func (m *Map[K, V]) Keys() []K {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	keys := make([]K, 0, len(m.items))
	for k, e := range m.items {
		if e.expiresAt.IsZero() || now.Before(e.expiresAt) {
			keys = append(keys, k)
		}
	}
	return keys
}

// Range calls fn for each non-expired item. If fn returns false, iteration stops.
func (m *Map[K, V]) Range(fn func(key K, value V) bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	for k, e := range m.items {
		if !e.expiresAt.IsZero() && now.After(e.expiresAt) {
			continue
		}
		if !fn(k, e.value) {
			break
		}
	}
}

// Clear removes all items.
func (m *Map[K, V]) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items = make(map[K]entry[V])
}

// Cleanup removes expired entries and returns the count removed.
func (m *Map[K, V]) Cleanup() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	removed := 0
	for k, e := range m.items {
		if !e.expiresAt.IsZero() && now.After(e.expiresAt) {
			delete(m.items, k)
			removed++
		}
	}
	return removed
}

// Values returns all non-expired values.
func (m *Map[K, V]) Values() []V {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	values := make([]V, 0, len(m.items))
	for _, e := range m.items {
		if e.expiresAt.IsZero() || now.Before(e.expiresAt) {
			values = append(values, e.value)
		}
	}
	return values
}
