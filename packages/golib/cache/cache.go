package cache

import (
	"sync"
	"sync/atomic"
	"time"
)

// Cache provides a thread-safe in-memory cache with TTL.
type Cache struct {
	items      map[string]*item
	mu         sync.RWMutex
	defaultTTL time.Duration
	hits       atomic.Int64
	misses     atomic.Int64
}

type item struct {
	value     interface{}
	expiresAt time.Time
}

// New creates a cache with the given default TTL.
func New(defaultTTL time.Duration) *Cache {
	return &Cache{
		items:      make(map[string]*item),
		defaultTTL: defaultTTL,
	}
}

// Get retrieves a value. Returns (value, true) if found and not expired,
// (nil, false) otherwise.
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	it, ok := c.items[key]
	c.mu.RUnlock()

	if !ok || time.Now().After(it.expiresAt) {
		c.misses.Add(1)
		return nil, false
	}

	c.hits.Add(1)
	return it.value, true
}

// Set stores a value with the default TTL.
func (c *Cache) Set(key string, value interface{}) {
	c.SetWithTTL(key, value, c.defaultTTL)
}

// SetWithTTL stores a value with a custom TTL.
func (c *Cache) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	c.items[key] = &item{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
	c.mu.Unlock()
}

// Delete removes a value.
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
}

// Clear removes all values.
func (c *Cache) Clear() {
	c.mu.Lock()
	c.items = make(map[string]*item)
	c.mu.Unlock()
}

// Size returns the number of non-expired items.
func (c *Cache) Size() int {
	now := time.Now()
	c.mu.RLock()
	defer c.mu.RUnlock()

	count := 0
	for _, it := range c.items {
		if now.Before(it.expiresAt) {
			count++
		}
	}
	return count
}

// GetOrSet retrieves value if cached, otherwise calls fn and caches result.
// If fn returns an error, the result is not cached.
func (c *Cache) GetOrSet(key string, fn func() (interface{}, error)) (interface{}, error) {
	if v, ok := c.Get(key); ok {
		return v, nil
	}

	v, err := fn()
	if err != nil {
		return nil, err
	}

	c.Set(key, v)
	return v, nil
}

// Cleanup removes expired items. Call periodically.
func (c *Cache) Cleanup() {
	now := time.Now()
	c.mu.Lock()
	for key, it := range c.items {
		if now.After(it.expiresAt) {
			delete(c.items, key)
		}
	}
	c.mu.Unlock()
}

// CacheStats holds cache hit/miss statistics.
type CacheStats struct {
	Hits   int64
	Misses int64
	Size   int
}

// Stats returns cache hit/miss statistics.
func (c *Cache) Stats() CacheStats {
	return CacheStats{
		Hits:   c.hits.Load(),
		Misses: c.misses.Load(),
		Size:   c.Size(),
	}
}
