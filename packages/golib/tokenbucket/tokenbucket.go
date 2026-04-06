// Package tokenbucket provides a token bucket with burst tracking
// and wait duration calculation. Enhanced version with metrics
// for monitoring bucket utilization and rejection rates.
package tokenbucket

import (
	"sync"
	"sync/atomic"
	"time"
)

// Bucket is a token bucket with metrics.
type Bucket struct {
	mu        sync.Mutex
	capacity  float64
	rate      float64
	tokens    float64
	lastTime  time.Time
	allowed   atomic.Int64
	rejected  atomic.Int64
}

// New creates a token bucket.
func New(capacity, rate float64) *Bucket {
	return &Bucket{
		capacity: capacity,
		rate:     rate,
		tokens:   capacity,
		lastTime: time.Now(),
	}
}

// Allow checks and consumes one token.
func (b *Bucket) Allow() bool {
	return b.AllowN(1)
}

// AllowN checks and consumes n tokens.
func (b *Bucket) AllowN(n float64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.refill()

	if b.tokens >= n {
		b.tokens -= n
		b.allowed.Add(1)
		return true
	}
	b.rejected.Add(1)
	return false
}

// Tokens returns the current token count.
func (b *Bucket) Tokens() float64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.refill()
	return b.tokens
}

// WaitDuration returns how long to wait for n tokens.
func (b *Bucket) WaitDuration(n float64) time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.refill()

	if b.tokens >= n {
		return 0
	}
	deficit := n - b.tokens
	return time.Duration(deficit / b.rate * float64(time.Second))
}

// Stats returns bucket usage statistics.
type Stats struct {
	Capacity    float64 `json:"capacity"`
	Rate        float64 `json:"rate"`
	Tokens      float64 `json:"tokens"`
	Utilization float64 `json:"utilization"` // 0-1
	Allowed     int64   `json:"allowed"`
	Rejected    int64   `json:"rejected"`
	RejectRate  float64 `json:"reject_rate"` // 0-1
}

// Stats returns current statistics.
func (b *Bucket) Stats() Stats {
	b.mu.Lock()
	b.refill()
	tokens := b.tokens
	b.mu.Unlock()

	allowed := b.allowed.Load()
	rejected := b.rejected.Load()
	total := allowed + rejected

	var rejectRate float64
	if total > 0 {
		rejectRate = float64(rejected) / float64(total)
	}

	return Stats{
		Capacity:    b.capacity,
		Rate:        b.rate,
		Tokens:      tokens,
		Utilization: 1 - (tokens / b.capacity),
		Allowed:     allowed,
		Rejected:    rejected,
		RejectRate:  rejectRate,
	}
}

// Reset restores to full capacity and clears metrics.
func (b *Bucket) Reset() {
	b.mu.Lock()
	b.tokens = b.capacity
	b.lastTime = time.Now()
	b.mu.Unlock()
	b.allowed.Store(0)
	b.rejected.Store(0)
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
