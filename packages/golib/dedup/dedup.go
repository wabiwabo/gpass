// Package dedup provides exactly-once processing guarantees for event
// consumers. It tracks processed event IDs with a configurable TTL
// to prevent duplicate processing in distributed systems.
package dedup

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// Store defines the interface for deduplication state persistence.
type Store interface {
	// Exists checks if a key has been processed.
	Exists(key string) (bool, error)
	// Mark records a key as processed with a TTL.
	Mark(key string, ttl time.Duration) error
	// Remove deletes a key (for reprocessing).
	Remove(key string) error
}

// Processor wraps event processing with deduplication.
type Processor struct {
	store Store
	ttl   time.Duration
}

// NewProcessor creates a deduplication processor.
func NewProcessor(store Store, ttl time.Duration) *Processor {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &Processor{store: store, ttl: ttl}
}

// Process runs fn only if the key hasn't been processed before.
// Returns (true, result) if processed, (false, nil) if duplicate.
func (p *Processor) Process(key string, fn func() error) (bool, error) {
	exists, err := p.store.Exists(key)
	if err != nil {
		return false, fmt.Errorf("dedup check: %w", err)
	}
	if exists {
		return false, nil // Duplicate — skip.
	}

	if err := fn(); err != nil {
		return false, err
	}

	if err := p.store.Mark(key, p.ttl); err != nil {
		// Processing succeeded but marking failed.
		// Return true since we did process it.
		return true, fmt.Errorf("dedup mark: %w", err)
	}

	return true, nil
}

// ContentKey generates a dedup key from content hash.
func ContentKey(eventType string, data []byte) string {
	h := sha256.New()
	h.Write([]byte(eventType))
	h.Write([]byte(":"))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// CompositeKey creates a key from multiple components.
func CompositeKey(parts ...string) string {
	h := sha256.New()
	for i, p := range parts {
		if i > 0 {
			h.Write([]byte(":"))
		}
		h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// MemoryStore is an in-memory dedup store for testing and development.
type MemoryStore struct {
	mu      sync.RWMutex
	entries map[string]time.Time
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		entries: make(map[string]time.Time),
	}
}

// Exists checks if a key exists and hasn't expired.
func (m *MemoryStore) Exists(key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	exp, ok := m.entries[key]
	if !ok {
		return false, nil
	}
	if time.Now().After(exp) {
		return false, nil // Expired.
	}
	return true, nil
}

// Mark records a key with expiry.
func (m *MemoryStore) Mark(key string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[key] = time.Now().Add(ttl)
	return nil
}

// Remove deletes a key.
func (m *MemoryStore) Remove(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.entries, key)
	return nil
}

// Size returns the number of tracked keys (including expired).
func (m *MemoryStore) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.entries)
}

// Cleanup removes expired entries.
func (m *MemoryStore) Cleanup() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	removed := 0
	for k, exp := range m.entries {
		if now.After(exp) {
			delete(m.entries, k)
			removed++
		}
	}
	return removed
}
