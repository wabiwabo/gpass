// Package cachettl provides an in-memory TTL cache with automatic
// expiration. Safe for concurrent use. Designed for caching hot data
// like session lookups, token validations, and config values.
package cachettl

import (
	"sync"
	"time"
)

type entry[V any] struct {
	value     V
	expiresAt time.Time
}

// Cache is a generic TTL cache.
type Cache[K comparable, V any] struct {
	mu      sync.RWMutex
	items   map[K]entry[V]
	defaultTTL time.Duration
}

// New creates a TTL cache with the given default TTL.
func New[K comparable, V any](defaultTTL time.Duration) *Cache[K, V] {
	return &Cache[K, V]{
		items:      make(map[K]entry[V]),
		defaultTTL: defaultTTL,
	}
}

// Set stores a value with the default TTL.
func (c *Cache[K, V]) Set(key K, value V) {
	c.SetWithTTL(key, value, c.defaultTTL)
}

// SetWithTTL stores a value with a specific TTL.
func (c *Cache[K, V]) SetWithTTL(key K, value V, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = entry[V]{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
}

// Get retrieves a value if it exists and hasn't expired.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	e, ok := c.items[key]
	c.mu.RUnlock()

	if !ok {
		var zero V
		return zero, false
	}

	if time.Now().After(e.expiresAt) {
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		var zero V
		return zero, false
	}

	return e.value, true
}

// GetOrSet retrieves a value, or calls fn to compute and store it.
func (c *Cache[K, V]) GetOrSet(key K, fn func() V) V {
	if v, ok := c.Get(key); ok {
		return v
	}

	value := fn()
	c.Set(key, value)
	return value
}

// Delete removes a key from the cache.
func (c *Cache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

// Has checks if a key exists and is not expired.
func (c *Cache[K, V]) Has(key K) bool {
	_, ok := c.Get(key)
	return ok
}

// Len returns the number of items (including expired).
func (c *Cache[K, V]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Purge removes all expired items. Returns count removed.
func (c *Cache[K, V]) Purge() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	removed := 0
	for k, e := range c.items {
		if now.After(e.expiresAt) {
			delete(c.items, k)
			removed++
		}
	}
	return removed
}

// Clear removes all items.
func (c *Cache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[K]entry[V])
}

// Keys returns all non-expired keys.
func (c *Cache[K, V]) Keys() []K {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	keys := make([]K, 0, len(c.items))
	for k, e := range c.items {
		if !now.After(e.expiresAt) {
			keys = append(keys, k)
		}
	}
	return keys
}
