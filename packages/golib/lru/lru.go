// Package lru provides a generic Least Recently Used (LRU) cache.
// Evicts the least recently accessed item when capacity is exceeded.
// Thread-safe for concurrent use.
package lru

import (
	"container/list"
	"sync"
)

type entry[K comparable, V any] struct {
	key   K
	value V
}

// Cache is a generic LRU cache.
type Cache[K comparable, V any] struct {
	mu       sync.Mutex
	capacity int
	items    map[K]*list.Element
	order    *list.List
	onEvict  func(K, V)
}

// Option configures the cache.
type Option[K comparable, V any] func(*Cache[K, V])

// WithOnEvict sets a callback when items are evicted.
func WithOnEvict[K comparable, V any](fn func(K, V)) Option[K, V] {
	return func(c *Cache[K, V]) { c.onEvict = fn }
}

// New creates an LRU cache with the given capacity.
func New[K comparable, V any](capacity int, opts ...Option[K, V]) *Cache[K, V] {
	if capacity <= 0 {
		capacity = 128
	}
	c := &Cache[K, V]{
		capacity: capacity,
		items:    make(map[K]*list.Element, capacity),
		order:    list.New(),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Get retrieves a value and marks it as recently used.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.order.MoveToFront(elem)
		return elem.Value.(*entry[K, V]).value, true
	}
	var zero V
	return zero, false
}

// Set adds or updates a value.
func (c *Cache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.order.MoveToFront(elem)
		elem.Value.(*entry[K, V]).value = value
		return
	}

	if c.order.Len() >= c.capacity {
		c.evictOldest()
	}

	elem := c.order.PushFront(&entry[K, V]{key: key, value: value})
	c.items[key] = elem
}

// Delete removes a key.
func (c *Cache[K, V]) Delete(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		return false
	}
	c.removeElement(elem)
	return true
}

// Has checks if a key exists (without updating recency).
func (c *Cache[K, V]) Has(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.items[key]
	return ok
}

// Len returns the number of items.
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

// Cap returns the capacity.
func (c *Cache[K, V]) Cap() int {
	return c.capacity
}

// Clear removes all items.
func (c *Cache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[K]*list.Element, c.capacity)
	c.order.Init()
}

// Keys returns all keys in order of most to least recently used.
func (c *Cache[K, V]) Keys() []K {
	c.mu.Lock()
	defer c.mu.Unlock()

	keys := make([]K, 0, c.order.Len())
	for e := c.order.Front(); e != nil; e = e.Next() {
		keys = append(keys, e.Value.(*entry[K, V]).key)
	}
	return keys
}

func (c *Cache[K, V]) evictOldest() {
	elem := c.order.Back()
	if elem == nil {
		return
	}
	c.removeElement(elem)
}

func (c *Cache[K, V]) removeElement(elem *list.Element) {
	c.order.Remove(elem)
	e := elem.Value.(*entry[K, V])
	delete(c.items, e.key)
	if c.onEvict != nil {
		c.onEvict(e.key, e.value)
	}
}
