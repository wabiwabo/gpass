// Package ratewindow provides fixed-window rate counting for analytics
// and monitoring. Unlike ratelimit (which enforces limits), this package
// counts events in time windows for metrics, alerting, and dashboards.
package ratewindow

import (
	"sync"
	"sync/atomic"
	"time"
)

// Counter tracks event counts in fixed time windows.
type Counter struct {
	mu       sync.RWMutex
	windows  map[string]*windowData
	duration time.Duration
}

type windowData struct {
	count   atomic.Int64
	start   time.Time
}

// NewCounter creates a counter with the given window duration.
func NewCounter(window time.Duration) *Counter {
	if window <= 0 {
		window = time.Minute
	}
	return &Counter{
		windows:  make(map[string]*windowData),
		duration: window,
	}
}

// Increment adds 1 to the key's current window count.
func (c *Counter) Increment(key string) int64 {
	return c.Add(key, 1)
}

// Add adds n to the key's current window count.
func (c *Counter) Add(key string, n int64) int64 {
	c.mu.RLock()
	w, ok := c.windows[key]
	c.mu.RUnlock()

	now := time.Now()

	if ok && now.Sub(w.start) < c.duration {
		return w.count.Add(n)
	}

	// Need new window.
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after lock.
	w, ok = c.windows[key]
	if ok && now.Sub(w.start) < c.duration {
		return w.count.Add(n)
	}

	newW := &windowData{start: now}
	newW.count.Store(n)
	c.windows[key] = newW
	return n
}

// Count returns the current count for a key in the active window.
func (c *Counter) Count(key string) int64 {
	c.mu.RLock()
	w, ok := c.windows[key]
	c.mu.RUnlock()

	if !ok {
		return 0
	}
	if time.Since(w.start) >= c.duration {
		return 0 // Window expired.
	}
	return w.count.Load()
}

// Rate returns the events per second for a key.
func (c *Counter) Rate(key string) float64 {
	c.mu.RLock()
	w, ok := c.windows[key]
	c.mu.RUnlock()

	if !ok {
		return 0
	}

	elapsed := time.Since(w.start)
	if elapsed >= c.duration {
		return 0
	}
	if elapsed <= 0 {
		return 0
	}

	return float64(w.count.Load()) / elapsed.Seconds()
}

// All returns all active key counts.
func (c *Counter) All() map[string]int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	result := make(map[string]int64)
	for k, w := range c.windows {
		if now.Sub(w.start) < c.duration {
			result[k] = w.count.Load()
		}
	}
	return result
}

// Reset clears all windows.
func (c *Counter) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.windows = make(map[string]*windowData)
}

// Cleanup removes expired windows.
func (c *Counter) Cleanup() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	removed := 0
	for k, w := range c.windows {
		if now.Sub(w.start) >= c.duration {
			delete(c.windows, k)
			removed++
		}
	}
	return removed
}

// Size returns the number of tracked keys.
func (c *Counter) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.windows)
}
