// Package ratebucket implements a token bucket rate limiter.
// Allows burst traffic up to bucket capacity while enforcing
// a sustained rate limit. Thread-safe for concurrent use.
package ratebucket

import (
	"sync"
	"time"
)

// Bucket is a token bucket rate limiter.
type Bucket struct {
	mu       sync.Mutex
	capacity float64       // max tokens
	rate     float64       // tokens per second
	tokens   float64       // current tokens
	lastTime time.Time
}

// New creates a token bucket with the given capacity and refill rate.
// Capacity is the maximum burst size.
// Rate is tokens added per second.
func New(capacity, rate float64) *Bucket {
	return &Bucket{
		capacity: capacity,
		rate:     rate,
		tokens:   capacity, // start full
		lastTime: time.Now(),
	}
}

// Allow checks if a single token is available and consumes it.
func (b *Bucket) Allow() bool {
	return b.AllowN(1)
}

// AllowN checks if n tokens are available and consumes them.
func (b *Bucket) AllowN(n float64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.refill()

	if b.tokens >= n {
		b.tokens -= n
		return true
	}
	return false
}

// Tokens returns the current number of available tokens.
func (b *Bucket) Tokens() float64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.refill()
	return b.tokens
}

// Reset restores the bucket to full capacity.
func (b *Bucket) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tokens = b.capacity
	b.lastTime = time.Now()
}

// Capacity returns the maximum number of tokens.
func (b *Bucket) Capacity() float64 {
	return b.capacity
}

// Rate returns the refill rate (tokens per second).
func (b *Bucket) Rate() float64 {
	return b.rate
}

func (b *Bucket) refill() {
	now := time.Now()
	elapsed := now.Sub(b.lastTime).Seconds()
	b.lastTime = now

	b.tokens += elapsed * b.rate
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}
}

// WaitTime returns how long to wait for n tokens to become available.
// Returns 0 if tokens are already available.
func (b *Bucket) WaitTime(n float64) time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.refill()

	if b.tokens >= n {
		return 0
	}

	deficit := n - b.tokens
	return time.Duration(deficit / b.rate * float64(time.Second))
}
