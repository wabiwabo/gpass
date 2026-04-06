package ratelimit

import (
	"sync"
	"time"
)

// SlidingWindow implements a sliding window counter rate limiter.
// It approximates a true sliding window by weighting the previous
// fixed window's count against the current window's elapsed time.
// This prevents boundary burst issues of fixed-window rate limiters.
//
// Algorithm: effective_count = prev_count * (1 - elapsed/window) + curr_count
// If effective_count + cost > limit, request is rejected.
type SlidingWindow struct {
	mu      sync.Mutex
	windows map[string]*windowState
	limit   int
	window  time.Duration
}

type windowState struct {
	prevCount    int
	currCount    int
	windowStart  time.Time
	windowSize   time.Duration
}

// NewSlidingWindow creates a sliding window rate limiter.
// limit: max requests per window. window: time window duration.
func NewSlidingWindow(limit int, window time.Duration) *SlidingWindow {
	return &SlidingWindow{
		windows: make(map[string]*windowState),
		limit:   limit,
		window:  window,
	}
}

// Allow checks if a request from the given key is within the rate limit.
func (sw *SlidingWindow) Allow(key string) bool {
	return sw.AllowN(key, 1)
}

// AllowN checks if n requests from the given key are within the rate limit.
func (sw *SlidingWindow) AllowN(key string, n int) bool {
	if n > sw.limit {
		return false
	}

	sw.mu.Lock()
	defer sw.mu.Unlock()

	now := time.Now()
	w, ok := sw.windows[key]
	if !ok {
		w = &windowState{
			windowStart: now,
			windowSize:  sw.window,
		}
		sw.windows[key] = w
	}

	// Advance windows if needed.
	elapsed := now.Sub(w.windowStart)
	if elapsed >= 2*sw.window {
		// Both windows have passed, reset everything.
		w.prevCount = 0
		w.currCount = 0
		w.windowStart = now
	} else if elapsed >= sw.window {
		// Current window has completed, shift.
		w.prevCount = w.currCount
		w.currCount = 0
		w.windowStart = w.windowStart.Add(sw.window)
	}

	// Calculate effective count using sliding window approximation.
	currentElapsed := now.Sub(w.windowStart)
	weight := 1.0 - float64(currentElapsed)/float64(sw.window)
	if weight < 0 {
		weight = 0
	}

	effectiveCount := float64(w.prevCount)*weight + float64(w.currCount)

	if effectiveCount+float64(n) > float64(sw.limit) {
		return false
	}

	w.currCount += n
	return true
}

// Remaining returns the approximate remaining requests for a key.
func (sw *SlidingWindow) Remaining(key string) int {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	now := time.Now()
	w, ok := sw.windows[key]
	if !ok {
		return sw.limit
	}

	elapsed := now.Sub(w.windowStart)
	if elapsed >= 2*sw.window {
		return sw.limit
	}

	// Shift if needed (read-only approximation).
	prevCount := w.prevCount
	currCount := w.currCount
	start := w.windowStart

	if elapsed >= sw.window {
		prevCount = currCount
		currCount = 0
		start = start.Add(sw.window)
	}

	currentElapsed := now.Sub(start)
	weight := 1.0 - float64(currentElapsed)/float64(sw.window)
	if weight < 0 {
		weight = 0
	}

	effectiveCount := float64(prevCount)*weight + float64(currCount)
	remaining := sw.limit - int(effectiveCount)
	if remaining < 0 {
		remaining = 0
	}
	return remaining
}

// Reset removes the state for a key.
func (sw *SlidingWindow) Reset(key string) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	delete(sw.windows, key)
}

// Cleanup removes stale window states older than 2 window periods.
func (sw *SlidingWindow) Cleanup() {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	cutoff := time.Now().Add(-2 * sw.window)
	for key, w := range sw.windows {
		if w.windowStart.Before(cutoff) {
			delete(sw.windows, key)
		}
	}
}

// Limit returns the configured request limit per window.
func (sw *SlidingWindow) Limit() int { return sw.limit }

// Window returns the configured time window.
func (sw *SlidingWindow) Window() time.Duration { return sw.window }
