// Package circuithttp provides a circuit-breaker-aware HTTP transport
// that automatically opens circuits on repeated failures and returns
// fast-fail responses when the circuit is open.
package circuithttp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// ErrCircuitOpen is returned when the circuit breaker is open.
var ErrCircuitOpen = errors.New("circuithttp: circuit open")

// State represents circuit breaker state.
type State int

const (
	StateClosed   State = iota // Normal operation.
	StateOpen                  // Failing fast.
	StateHalfOpen              // Testing recovery.
)

// String returns the state name.
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// Config configures the circuit breaker transport.
type Config struct {
	FailureThreshold int           // Failures before opening.
	SuccessThreshold int           // Successes in half-open to close.
	OpenDuration     time.Duration // How long to stay open.
	HalfOpenMax      int           // Max concurrent requests in half-open.
	IsFailure        func(resp *http.Response, err error) bool // Custom failure classifier.
}

// DefaultConfig returns production defaults.
func DefaultConfig() Config {
	return Config{
		FailureThreshold: 5,
		SuccessThreshold: 3,
		OpenDuration:     30 * time.Second,
		HalfOpenMax:      1,
	}
}

// Transport wraps http.RoundTripper with circuit breaker logic.
type Transport struct {
	Base   http.RoundTripper
	config Config

	mu             sync.Mutex
	state          State
	failures       int
	successes      int
	lastFailure    time.Time
	halfOpenActive int

	totalReqs   atomic.Int64
	failedReqs  atomic.Int64
	trippedReqs atomic.Int64
	onStateChange func(from, to State)
}

// NewTransport creates a circuit-breaker-aware transport.
func NewTransport(base http.RoundTripper, cfg Config) *Transport {
	if base == nil {
		base = http.DefaultTransport
	}
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 5
	}
	if cfg.SuccessThreshold <= 0 {
		cfg.SuccessThreshold = 3
	}
	if cfg.OpenDuration <= 0 {
		cfg.OpenDuration = 30 * time.Second
	}
	if cfg.HalfOpenMax <= 0 {
		cfg.HalfOpenMax = 1
	}
	if cfg.IsFailure == nil {
		cfg.IsFailure = defaultIsFailure
	}

	return &Transport{
		Base:   base,
		config: cfg,
	}
}

// OnStateChange sets a callback for circuit state transitions.
func (t *Transport) OnStateChange(fn func(from, to State)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onStateChange = fn
}

// RoundTrip implements http.RoundTripper with circuit breaker.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.totalReqs.Add(1)

	if !t.canProceed() {
		t.trippedReqs.Add(1)
		return nil, ErrCircuitOpen
	}

	resp, err := t.Base.RoundTrip(req)

	if t.config.IsFailure(resp, err) {
		t.recordFailure()
		t.failedReqs.Add(1)
		return resp, err
	}

	t.recordSuccess()
	return resp, err
}

func (t *Transport) canProceed() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	switch t.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(t.lastFailure) >= t.config.OpenDuration {
			t.transitionLocked(StateHalfOpen)
			t.halfOpenActive = 1
			return true
		}
		return false
	case StateHalfOpen:
		if t.halfOpenActive < t.config.HalfOpenMax {
			t.halfOpenActive++
			return true
		}
		return false
	}
	return false
}

func (t *Transport) recordSuccess() {
	t.mu.Lock()
	defer t.mu.Unlock()

	switch t.state {
	case StateHalfOpen:
		t.successes++
		t.halfOpenActive--
		if t.successes >= t.config.SuccessThreshold {
			t.transitionLocked(StateClosed)
		}
	case StateClosed:
		t.failures = 0 // Reset consecutive failures on success.
	}
}

func (t *Transport) recordFailure() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.lastFailure = time.Now()

	switch t.state {
	case StateClosed:
		t.failures++
		if t.failures >= t.config.FailureThreshold {
			t.transitionLocked(StateOpen)
		}
	case StateHalfOpen:
		t.halfOpenActive--
		t.transitionLocked(StateOpen) // One failure in half-open → reopen.
	}
}

func (t *Transport) transitionLocked(to State) {
	if t.state == to {
		return
	}
	from := t.state
	t.state = to
	t.failures = 0
	t.successes = 0

	if t.onStateChange != nil {
		fn := t.onStateChange
		go fn(from, to)
	}
}

// State returns the current circuit state.
func (t *Transport) State() State {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state
}

// Stats returns circuit breaker statistics.
func (t *Transport) Stats() Stats {
	t.mu.Lock()
	state := t.state
	failures := t.failures
	t.mu.Unlock()

	return Stats{
		State:            state,
		TotalRequests:    t.totalReqs.Load(),
		FailedRequests:   t.failedReqs.Load(),
		TrippedRequests:  t.trippedReqs.Load(),
		ConsecFailures:   failures,
	}
}

// Reset forces the circuit back to closed state.
func (t *Transport) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.transitionLocked(StateClosed)
}

// Stats holds circuit breaker telemetry.
type Stats struct {
	State           State `json:"state"`
	TotalRequests   int64 `json:"total_requests"`
	FailedRequests  int64 `json:"failed_requests"`
	TrippedRequests int64 `json:"tripped_requests"`
	ConsecFailures  int   `json:"consecutive_failures"`
}

func defaultIsFailure(resp *http.Response, err error) bool {
	if err != nil {
		return true
	}
	if resp != nil && resp.StatusCode >= 500 {
		return true
	}
	return false
}

// Client returns an *http.Client using this circuit-breaker transport.
func (t *Transport) Client() *http.Client {
	return &http.Client{Transport: t}
}

// Do performs a request through the circuit breaker.
func (t *Transport) Do(ctx context.Context, method, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("circuithttp: create request: %w", err)
	}
	return t.RoundTrip(req)
}
