// Package countx provides thread-safe counters and accumulators.
// Includes atomic counters, windowed counters for rate calculation,
// and labeled counter groups for metrics.
package countx

import (
	"sort"
	"sync"
	"sync/atomic"
)

// Counter is an atomic counter.
type Counter struct {
	v atomic.Int64
}

// Inc increments by 1.
func (c *Counter) Inc() int64 {
	return c.v.Add(1)
}

// Add adds n to the counter.
func (c *Counter) Add(n int64) int64 {
	return c.v.Add(n)
}

// Value returns the current count.
func (c *Counter) Value() int64 {
	return c.v.Load()
}

// Reset sets the counter to zero and returns the old value.
func (c *Counter) Reset() int64 {
	return c.v.Swap(0)
}

// Group is a collection of named counters.
type Group struct {
	mu       sync.RWMutex
	counters map[string]*Counter
}

// NewGroup creates a counter group.
func NewGroup() *Group {
	return &Group{counters: make(map[string]*Counter)}
}

// Inc increments a named counter.
func (g *Group) Inc(name string) int64 {
	return g.Add(name, 1)
}

// Add adds n to a named counter.
func (g *Group) Add(name string, n int64) int64 {
	g.mu.RLock()
	c, ok := g.counters[name]
	g.mu.RUnlock()

	if ok {
		return c.Add(n)
	}

	g.mu.Lock()
	c, ok = g.counters[name]
	if !ok {
		c = &Counter{}
		g.counters[name] = c
	}
	g.mu.Unlock()
	return c.Add(n)
}

// Get returns the value of a named counter.
func (g *Group) Get(name string) int64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	c, ok := g.counters[name]
	if !ok {
		return 0
	}
	return c.Value()
}

// All returns all counter values sorted by name.
func (g *Group) All() map[string]int64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	result := make(map[string]int64, len(g.counters))
	for name, c := range g.counters {
		result[name] = c.Value()
	}
	return result
}

// Names returns all counter names sorted.
func (g *Group) Names() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	names := make([]string, 0, len(g.counters))
	for name := range g.counters {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Reset resets all counters and returns their values.
func (g *Group) Reset() map[string]int64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	result := make(map[string]int64, len(g.counters))
	for name, c := range g.counters {
		result[name] = c.Reset()
	}
	return result
}

// Len returns the number of counters.
func (g *Group) Len() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.counters)
}
