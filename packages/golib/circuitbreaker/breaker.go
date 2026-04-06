package circuitbreaker

import (
	"sync"
	"time"
)

const (
	StateClosed   = "closed"
	StateOpen     = "open"
	StateHalfOpen = "half-open"
)

// Breaker implements a thread-safe circuit breaker pattern.
type Breaker struct {
	mu        sync.Mutex
	threshold int
	cooldown  time.Duration
	failures  int
	state     string
	openedAt  time.Time
}

// New creates a new circuit breaker that opens after threshold consecutive
// failures and transitions to half-open after cooldown duration.
func New(threshold int, cooldown time.Duration) *Breaker {
	return &Breaker{
		threshold: threshold,
		cooldown:  cooldown,
		state:     StateClosed,
	}
}

// Allow reports whether the request is allowed.
// In closed state, always allows.
// In open state, allows only if cooldown has elapsed (transitions to half-open).
// In half-open state, allows one probe request.
func (b *Breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(b.openedAt) >= b.cooldown {
			b.state = StateHalfOpen
			return true
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return true
	}
}

// RecordSuccess records a successful call. Resets failures and moves to closed.
func (b *Breaker) RecordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.state = StateClosed
}

// RecordFailure records a failed call. Increments the failure counter and
// opens the circuit if the threshold is reached.
func (b *Breaker) RecordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	if b.failures >= b.threshold {
		b.state = StateOpen
		b.openedAt = time.Now()
	}
}

// State returns the current state of the circuit breaker.
func (b *Breaker) State() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// FailureCount returns the current failure count.
func (b *Breaker) FailureCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.failures
}

// Threshold returns the failure threshold.
func (b *Breaker) Threshold() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.threshold
}

// OpenedAt returns the time when the circuit breaker was last opened.
// Returns zero time if the breaker has never been opened.
func (b *Breaker) OpenedAt() time.Time {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.openedAt
}
