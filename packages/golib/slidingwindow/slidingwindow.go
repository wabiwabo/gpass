// Package slidingwindow implements a sliding window rate limiter.
// Counts requests within a rolling time window, providing more
// accurate rate limiting than fixed windows at the cost of memory.
package slidingwindow

import (
	"sync"
	"time"
)

// Window is a sliding window rate limiter.
type Window struct {
	mu        sync.Mutex
	limit     int
	window    time.Duration
	requests  []time.Time
}

// New creates a sliding window rate limiter.
// Limit is the maximum requests per window.
// Window is the time duration of the window.
func New(limit int, window time.Duration) *Window {
	return &Window{
		limit:    limit,
		window:   window,
		requests: make([]time.Time, 0, limit),
	}
}

// Allow checks if a request is within the rate limit.
func (w *Window) Allow() bool {
	return w.AllowAt(time.Now())
}

// AllowAt checks if a request at the given time is within the limit.
func (w *Window) AllowAt(now time.Time) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.evict(now)

	if len(w.requests) >= w.limit {
		return false
	}

	w.requests = append(w.requests, now)
	return true
}

// Count returns the number of requests in the current window.
func (w *Window) Count() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.evict(time.Now())
	return len(w.requests)
}

// Remaining returns how many more requests are allowed.
func (w *Window) Remaining() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.evict(time.Now())
	r := w.limit - len(w.requests)
	if r < 0 {
		return 0
	}
	return r
}

// Reset clears all recorded requests.
func (w *Window) Reset() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.requests = w.requests[:0]
}

// Limit returns the configured limit.
func (w *Window) Limit() int {
	return w.limit
}

// WindowDuration returns the window size.
func (w *Window) WindowDuration() time.Duration {
	return w.window
}

// NextAllowed returns when the next request will be allowed.
// Returns zero time if requests are already available.
func (w *Window) NextAllowed() time.Time {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.evict(time.Now())

	if len(w.requests) < w.limit {
		return time.Time{}
	}

	// Oldest request + window = when it expires
	return w.requests[0].Add(w.window)
}

func (w *Window) evict(now time.Time) {
	cutoff := now.Add(-w.window)
	i := 0
	for i < len(w.requests) && w.requests[i].Before(cutoff) {
		i++
	}
	if i > 0 {
		copy(w.requests, w.requests[i:])
		w.requests = w.requests[:len(w.requests)-i]
	}
}
