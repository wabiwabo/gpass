package adaptive

import (
	"sync"
	"sync/atomic"
	"time"
)

// Throttle implements adaptive rate limiting that adjusts based on error rates.
// When errors increase, the allowed rate decreases (backoff).
// When the system is healthy, the rate recovers toward the maximum.
type Throttle struct {
	mu sync.Mutex

	maxRate     float64 // maximum requests per second
	minRate     float64 // minimum requests per second (floor)
	currentRate float64 // current effective rate
	backoffRate float64 // multiplier when backing off (e.g., 0.5 = halve)
	recoverRate float64 // multiplier when recovering (e.g., 1.1 = +10%)

	window       time.Duration
	windowStart  time.Time
	windowTotal  int64
	windowErrors int64

	errorThreshold float64 // error ratio that triggers backoff (0-1)

	// Metrics
	totalAllowed atomic.Int64
	totalDenied  atomic.Int64
	totalBackoff atomic.Int64

	// Token bucket state
	tokens   float64
	lastFill time.Time
}

// Config configures the adaptive throttle.
type Config struct {
	// MaxRate is the maximum requests per second.
	MaxRate float64
	// MinRate is the minimum rate floor. Default: 1.
	MinRate float64
	// BackoffMultiplier reduces rate when errors spike. Default: 0.5.
	BackoffMultiplier float64
	// RecoverMultiplier increases rate when healthy. Default: 1.1.
	RecoverMultiplier float64
	// ErrorThreshold is the error ratio (0-1) that triggers backoff. Default: 0.1.
	ErrorThreshold float64
	// Window is the measurement window. Default: 10 seconds.
	Window time.Duration
}

// New creates a new adaptive throttle.
func New(cfg Config) *Throttle {
	if cfg.MinRate <= 0 {
		cfg.MinRate = 1
	}
	if cfg.BackoffMultiplier <= 0 || cfg.BackoffMultiplier >= 1 {
		cfg.BackoffMultiplier = 0.5
	}
	if cfg.RecoverMultiplier <= 1 {
		cfg.RecoverMultiplier = 1.1
	}
	if cfg.ErrorThreshold <= 0 {
		cfg.ErrorThreshold = 0.1
	}
	if cfg.Window <= 0 {
		cfg.Window = 10 * time.Second
	}

	return &Throttle{
		maxRate:        cfg.MaxRate,
		minRate:        cfg.MinRate,
		currentRate:    cfg.MaxRate,
		backoffRate:    cfg.BackoffMultiplier,
		recoverRate:    cfg.RecoverMultiplier,
		errorThreshold: cfg.ErrorThreshold,
		window:         cfg.Window,
		windowStart:    time.Now(),
		tokens:         cfg.MaxRate,
		lastFill:       time.Now(),
	}
}

// Allow checks if a request should be allowed under current rate.
func (t *Throttle) Allow() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.refillTokens()
	t.maybeAdjustRate()

	if t.tokens >= 1 {
		t.tokens--
		t.windowTotal++
		t.totalAllowed.Add(1)
		return true
	}

	t.totalDenied.Add(1)
	return false
}

// RecordSuccess records a successful request.
func (t *Throttle) RecordSuccess() {
	// No-op for success — only errors matter for adaptation.
}

// RecordError records a failed request, contributing to backoff decision.
func (t *Throttle) RecordError() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.windowErrors++
}

func (t *Throttle) refillTokens() {
	now := time.Now()
	elapsed := now.Sub(t.lastFill).Seconds()
	t.lastFill = now

	t.tokens += elapsed * t.currentRate
	if t.tokens > t.currentRate {
		t.tokens = t.currentRate
	}
}

func (t *Throttle) maybeAdjustRate() {
	now := time.Now()
	if now.Sub(t.windowStart) < t.window {
		return
	}

	// Window expired — evaluate and adjust.
	if t.windowTotal > 0 {
		errorRate := float64(t.windowErrors) / float64(t.windowTotal)
		if errorRate >= t.errorThreshold {
			// Back off.
			t.currentRate *= t.backoffRate
			if t.currentRate < t.minRate {
				t.currentRate = t.minRate
			}
			t.totalBackoff.Add(1)
		} else {
			// Recover.
			t.currentRate *= t.recoverRate
			if t.currentRate > t.maxRate {
				t.currentRate = t.maxRate
			}
		}
	}

	// Reset window.
	t.windowStart = now
	t.windowTotal = 0
	t.windowErrors = 0
}

// Stats returns current throttle statistics.
type Stats struct {
	CurrentRate float64 `json:"current_rate"`
	MaxRate     float64 `json:"max_rate"`
	MinRate     float64 `json:"min_rate"`
	Allowed     int64   `json:"total_allowed"`
	Denied      int64   `json:"total_denied"`
	Backoffs    int64   `json:"total_backoffs"`
}

// Stats returns current throttle stats.
func (t *Throttle) Stats() Stats {
	t.mu.Lock()
	rate := t.currentRate
	t.mu.Unlock()

	return Stats{
		CurrentRate: rate,
		MaxRate:     t.maxRate,
		MinRate:     t.minRate,
		Allowed:     t.totalAllowed.Load(),
		Denied:      t.totalDenied.Load(),
		Backoffs:    t.totalBackoff.Load(),
	}
}

// CurrentRate returns the current effective rate.
func (t *Throttle) CurrentRate() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.currentRate
}
