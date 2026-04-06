// Package leakybucket implements a leaky bucket rate limiter.
// Requests are processed at a fixed rate. Excess requests queue up
// to the bucket capacity, then are rejected.
package leakybucket

import (
	"sync"
	"time"
)

// Bucket is a leaky bucket rate limiter.
type Bucket struct {
	mu       sync.Mutex
	capacity int           // max queue size
	rate     time.Duration // time between drips
	queue    int           // current queue depth
	lastDrip time.Time
}

// New creates a leaky bucket.
// Capacity is the max queue depth.
// Rate is the interval between allowing requests.
func New(capacity int, rate time.Duration) *Bucket {
	return &Bucket{
		capacity: capacity,
		rate:     rate,
		lastDrip: time.Now(),
	}
}

// Allow checks if a request can be queued.
func (b *Bucket) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.drip()

	if b.queue < b.capacity {
		b.queue++
		return true
	}
	return false
}

// Queue returns the current queue depth.
func (b *Bucket) Queue() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.drip()
	return b.queue
}

// Reset empties the queue.
func (b *Bucket) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.queue = 0
	b.lastDrip = time.Now()
}

// Capacity returns the maximum queue depth.
func (b *Bucket) Capacity() int {
	return b.capacity
}

func (b *Bucket) drip() {
	now := time.Now()
	elapsed := now.Sub(b.lastDrip)

	drips := int(elapsed / b.rate)
	if drips > 0 {
		b.queue -= drips
		if b.queue < 0 {
			b.queue = 0
		}
		b.lastDrip = b.lastDrip.Add(time.Duration(drips) * b.rate)
	}
}
