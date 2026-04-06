// Package circuitkey provides named circuit breaker registry.
// Maps service/endpoint names to circuit breaker states for
// per-route resilience in microservice communication.
package circuitkey

import (
	"fmt"
	"sync"
	"time"
)

// State represents circuit breaker state.
type State int

const (
	Closed   State = iota // normal operation
	Open                  // rejecting requests
	HalfOpen              // testing recovery
)

func (s State) String() string {
	switch s {
	case Closed:
		return "closed"
	case Open:
		return "open"
	case HalfOpen:
		return "half-open"
	}
	return "unknown"
}

// Config controls circuit breaker behavior.
type Config struct {
	Threshold   int           // failures before opening
	Timeout     time.Duration // time in open state before half-open
	MaxHalfOpen int           // max requests in half-open state
}

// DefaultConfig returns conservative defaults.
func DefaultConfig() Config {
	return Config{
		Threshold:   5,
		Timeout:     30 * time.Second,
		MaxHalfOpen: 1,
	}
}

type breaker struct {
	mu           sync.Mutex
	state        State
	failures     int
	successes    int
	lastFailure  time.Time
	cfg          Config
}

// Registry manages named circuit breakers.
type Registry struct {
	mu       sync.RWMutex
	breakers map[string]*breaker
	cfg      Config
}

// NewRegistry creates a registry with the given default config.
func NewRegistry(cfg Config) *Registry {
	if cfg.Threshold <= 0 {
		cfg.Threshold = 5
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.MaxHalfOpen <= 0 {
		cfg.MaxHalfOpen = 1
	}
	return &Registry{
		breakers: make(map[string]*breaker),
		cfg:      cfg,
	}
}

func (r *Registry) get(key string) *breaker {
	r.mu.RLock()
	b, ok := r.breakers[key]
	r.mu.RUnlock()
	if ok {
		return b
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if b, ok := r.breakers[key]; ok {
		return b
	}
	b = &breaker{cfg: r.cfg}
	r.breakers[key] = b
	return b
}

// Allow checks if a request to the named service should proceed.
func (r *Registry) Allow(key string) bool {
	b := r.get(key)
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case Closed:
		return true
	case Open:
		if time.Since(b.lastFailure) > b.cfg.Timeout {
			b.state = HalfOpen
			b.successes = 0
			return true
		}
		return false
	case HalfOpen:
		return b.successes < b.cfg.MaxHalfOpen
	}
	return false
}

// RecordSuccess records a successful request.
func (r *Registry) RecordSuccess(key string) {
	b := r.get(key)
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case HalfOpen:
		b.successes++
		if b.successes >= b.cfg.MaxHalfOpen {
			b.state = Closed
			b.failures = 0
		}
	case Closed:
		b.failures = 0
	}
}

// RecordFailure records a failed request.
func (r *Registry) RecordFailure(key string) {
	b := r.get(key)
	b.mu.Lock()
	defer b.mu.Unlock()

	b.lastFailure = time.Now()

	switch b.state {
	case Closed:
		b.failures++
		if b.failures >= b.cfg.Threshold {
			b.state = Open
		}
	case HalfOpen:
		b.state = Open
	}
}

// GetState returns the current state of a named circuit breaker.
func (r *Registry) GetState(key string) State {
	b := r.get(key)
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// Reset resets a named circuit breaker to closed state.
func (r *Registry) Reset(key string) {
	b := r.get(key)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.state = Closed
	b.failures = 0
	b.successes = 0
}

// Keys returns all registered circuit breaker names.
func (r *Registry) Keys() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	keys := make([]string, 0, len(r.breakers))
	for k := range r.breakers {
		keys = append(keys, k)
	}
	return keys
}

// Status returns a formatted status for all breakers.
func (r *Registry) Status() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	status := make(map[string]string, len(r.breakers))
	for k, b := range r.breakers {
		b.mu.Lock()
		status[k] = fmt.Sprintf("state=%s failures=%d", b.state, b.failures)
		b.mu.Unlock()
	}
	return status
}
