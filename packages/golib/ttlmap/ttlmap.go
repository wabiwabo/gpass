// Package ttlmap provides a simple string-keyed TTL map.
// Lighter than cachettl for cases where generic types aren't needed.
// Thread-safe with automatic lazy expiration on access.
package ttlmap

import (
	"sync"
	"time"
)

type entry struct {
	value     string
	expiresAt time.Time
}

// Map is a string-to-string TTL map.
type Map struct {
	mu  sync.RWMutex
	m   map[string]entry
	ttl time.Duration
}

// New creates a TTL map with the given default TTL.
func New(ttl time.Duration) *Map {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &Map{
		m:   make(map[string]entry),
		ttl: ttl,
	}
}

// Set stores a value with the default TTL.
func (m *Map) Set(key, value string) {
	m.SetWithTTL(key, value, m.ttl)
}

// SetWithTTL stores a value with a specific TTL.
func (m *Map) SetWithTTL(key, value string, ttl time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.m[key] = entry{value: value, expiresAt: time.Now().Add(ttl)}
}

// Get returns a value if not expired.
func (m *Map) Get(key string) (string, bool) {
	m.mu.RLock()
	e, ok := m.m[key]
	m.mu.RUnlock()

	if !ok {
		return "", false
	}
	if time.Now().After(e.expiresAt) {
		m.mu.Lock()
		delete(m.m, key)
		m.mu.Unlock()
		return "", false
	}
	return e.value, true
}

// Delete removes a key.
func (m *Map) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.m, key)
}

// Has checks if a key exists and is not expired.
func (m *Map) Has(key string) bool {
	_, ok := m.Get(key)
	return ok
}

// Len returns the number of entries (including potentially expired).
func (m *Map) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.m)
}

// Purge removes expired entries. Returns count removed.
func (m *Map) Purge() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	removed := 0
	for k, e := range m.m {
		if now.After(e.expiresAt) {
			delete(m.m, k)
			removed++
		}
	}
	return removed
}

// Clear removes all entries.
func (m *Map) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.m = make(map[string]entry)
}
