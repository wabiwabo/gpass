// Package ratelimit_key provides per-key rate limiting using
// token buckets. Each unique key (user, IP, API key) gets its
// own rate limit with automatic cleanup of inactive keys.
package ratelimit_key

import (
	"sync"
	"time"
)

type bucket struct {
	tokens   float64
	lastTime time.Time
}

// Limiter manages per-key rate limits.
type Limiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     float64       // tokens per second
	capacity float64       // max tokens (burst)
	ttl      time.Duration // cleanup inactive keys after
}

// New creates a per-key rate limiter.
// Rate is requests per second, burst is max burst size.
func New(rate float64, burst int, ttl time.Duration) *Limiter {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &Limiter{
		buckets:  make(map[string]*bucket),
		rate:     rate,
		capacity: float64(burst),
		ttl:      ttl,
	}
}

// Allow checks if a request for the given key is allowed.
func (l *Limiter) Allow(key string) bool {
	return l.AllowN(key, 1)
}

// AllowN checks if n requests for the key are allowed.
func (l *Limiter) AllowN(key string, n float64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.capacity, lastTime: time.Now()}
		l.buckets[key] = b
	}

	// Refill tokens
	now := time.Now()
	elapsed := now.Sub(b.lastTime).Seconds()
	b.tokens += elapsed * l.rate
	if b.tokens > l.capacity {
		b.tokens = l.capacity
	}
	b.lastTime = now

	if b.tokens >= n {
		b.tokens -= n
		return true
	}
	return false
}

// Remaining returns the remaining tokens for a key.
func (l *Limiter) Remaining(key string) float64 {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok {
		return l.capacity
	}

	elapsed := time.Since(b.lastTime).Seconds()
	tokens := b.tokens + elapsed*l.rate
	if tokens > l.capacity {
		tokens = l.capacity
	}
	return tokens
}

// Reset resets the rate limit for a key.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.buckets, key)
}

// Purge removes inactive keys. Returns count removed.
func (l *Limiter) Purge() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	removed := 0
	for key, b := range l.buckets {
		if now.Sub(b.lastTime) > l.ttl {
			delete(l.buckets, key)
			removed++
		}
	}
	return removed
}

// Count returns the number of tracked keys.
func (l *Limiter) Count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}
