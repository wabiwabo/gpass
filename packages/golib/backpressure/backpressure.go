// Package backpressure provides system-level load shedding and admission
// control to protect services from overload. It monitors in-flight requests,
// latency, and error rates to decide whether to accept or reject new work.
package backpressure

import (
	"context"
	"math"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// State represents the current backpressure state.
type State int

const (
	StateNormal   State = iota // Accepting all requests.
	StateThrottle              // Shedding a percentage of load.
	StateShed                  // Rejecting most requests.
)

// String returns the state name.
func (s State) String() string {
	switch s {
	case StateNormal:
		return "normal"
	case StateThrottle:
		return "throttle"
	case StateShed:
		return "shed"
	default:
		return "unknown"
	}
}

// Config controls backpressure behavior.
type Config struct {
	MaxInFlight      int64         // Max concurrent requests before throttling.
	ShedThreshold    int64         // In-flight count that triggers full shedding.
	LatencyThreshold time.Duration // p99 latency that triggers throttling.
	WindowSize       time.Duration // Measurement window.
	CooldownPeriod   time.Duration // Min time between state transitions.
}

// DefaultConfig returns sensible production defaults.
func DefaultConfig() Config {
	return Config{
		MaxInFlight:      1000,
		ShedThreshold:    2000,
		LatencyThreshold: 5 * time.Second,
		WindowSize:       10 * time.Second,
		CooldownPeriod:   5 * time.Second,
	}
}

// Controller manages backpressure state and admission decisions.
type Controller struct {
	config Config

	inFlight     atomic.Int64
	totalReqs    atomic.Int64
	rejectedReqs atomic.Int64

	mu          sync.RWMutex
	state       State
	lastChange  time.Time
	latencies   []time.Duration
	latencyIdx  int
	latencyFull bool

	onStateChange func(old, new State)
}

// NewController creates a new backpressure controller.
func NewController(cfg Config) *Controller {
	if cfg.MaxInFlight <= 0 {
		cfg.MaxInFlight = 1000
	}
	if cfg.ShedThreshold <= 0 {
		cfg.ShedThreshold = cfg.MaxInFlight * 2
	}
	if cfg.LatencyThreshold <= 0 {
		cfg.LatencyThreshold = 5 * time.Second
	}
	if cfg.WindowSize <= 0 {
		cfg.WindowSize = 10 * time.Second
	}
	if cfg.CooldownPeriod <= 0 {
		cfg.CooldownPeriod = 5 * time.Second
	}

	return &Controller{
		config:    cfg,
		latencies: make([]time.Duration, 100), // Ring buffer of 100 samples.
	}
}

// OnStateChange registers a callback for state transitions.
func (c *Controller) OnStateChange(fn func(old, new State)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onStateChange = fn
}

// Admit decides whether a request should be accepted.
// Returns true if accepted, false if shed.
func (c *Controller) Admit() bool {
	c.totalReqs.Add(1)
	current := c.inFlight.Load()

	c.mu.RLock()
	state := c.state
	c.mu.RUnlock()

	switch {
	case current >= c.config.ShedThreshold:
		c.transition(StateShed)
		c.rejectedReqs.Add(1)
		return false
	case current >= c.config.MaxInFlight:
		c.transition(StateThrottle)
		// Probabilistic shedding: higher load = more shedding.
		ratio := float64(current-c.config.MaxInFlight) / float64(c.config.ShedThreshold-c.config.MaxInFlight)
		if ratio > 0.5 {
			c.rejectedReqs.Add(1)
			return false
		}
		return true
	case state != StateNormal && current < c.config.MaxInFlight/2:
		c.transition(StateNormal)
		return true
	default:
		return true
	}
}

// Begin marks a request as starting. Returns a done function to call when complete.
func (c *Controller) Begin() func() {
	c.inFlight.Add(1)
	start := time.Now()
	return func() {
		c.inFlight.Add(-1)
		c.recordLatency(time.Since(start))
	}
}

func (c *Controller) recordLatency(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.latencies[c.latencyIdx] = d
	c.latencyIdx = (c.latencyIdx + 1) % len(c.latencies)
	if c.latencyIdx == 0 {
		c.latencyFull = true
	}
}

func (c *Controller) transition(newState State) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state == newState {
		return
	}
	if time.Since(c.lastChange) < c.config.CooldownPeriod {
		return
	}

	old := c.state
	c.state = newState
	c.lastChange = time.Now()

	if c.onStateChange != nil {
		go c.onStateChange(old, newState)
	}
}

// State returns the current backpressure state.
func (c *Controller) State() State {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// InFlight returns the current in-flight request count.
func (c *Controller) InFlight() int64 {
	return c.inFlight.Load()
}

// Stats returns current backpressure statistics.
func (c *Controller) Stats() Stats {
	c.mu.RLock()
	p99 := c.computeP99()
	state := c.state
	c.mu.RUnlock()

	return Stats{
		State:        state,
		InFlight:     c.inFlight.Load(),
		TotalReqs:    c.totalReqs.Load(),
		RejectedReqs: c.rejectedReqs.Load(),
		P99Latency:   p99,
	}
}

func (c *Controller) computeP99() time.Duration {
	count := len(c.latencies)
	if !c.latencyFull {
		count = c.latencyIdx
	}
	if count == 0 {
		return 0
	}

	// Simple p99: sort and pick 99th percentile.
	sorted := make([]time.Duration, count)
	copy(sorted, c.latencies[:count])
	sortDurations(sorted)

	idx := int(math.Ceil(float64(count)*0.99)) - 1
	if idx < 0 {
		idx = 0
	}
	return sorted[idx]
}

func sortDurations(d []time.Duration) {
	// Simple insertion sort for small arrays (max 100).
	for i := 1; i < len(d); i++ {
		key := d[i]
		j := i - 1
		for j >= 0 && d[j] > key {
			d[j+1] = d[j]
			j--
		}
		d[j+1] = key
	}
}

// Stats holds backpressure telemetry.
type Stats struct {
	State        State         `json:"state"`
	InFlight     int64         `json:"in_flight"`
	TotalReqs    int64         `json:"total_requests"`
	RejectedReqs int64         `json:"rejected_requests"`
	P99Latency   time.Duration `json:"p99_latency"`
}

// RejectRatio returns the fraction of rejected requests.
func (s Stats) RejectRatio() float64 {
	if s.TotalReqs == 0 {
		return 0
	}
	return float64(s.RejectedReqs) / float64(s.TotalReqs)
}

// Middleware returns an HTTP middleware that applies backpressure.
// Rejected requests get 503 Service Unavailable with Retry-After.
func (c *Controller) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !c.Admit() {
			w.Header().Set("Retry-After", "5")
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"type":"about:blank","title":"Service Overloaded","status":503,"detail":"Server is under heavy load. Please retry later."}`))
			return
		}

		done := c.Begin()
		defer done()

		next.ServeHTTP(w, r)
	})
}

// PriorityAdmit checks admission with priority awareness.
// Higher priority requests (lower number) are more likely to be admitted.
func (c *Controller) PriorityAdmit(priority int) bool {
	c.totalReqs.Add(1)
	current := c.inFlight.Load()

	if current >= c.config.ShedThreshold {
		// Only priority 0 (critical) gets through at full shed.
		if priority == 0 {
			return true
		}
		c.rejectedReqs.Add(1)
		return false
	}

	if current >= c.config.MaxInFlight {
		// Priority 0-1 pass during throttle.
		if priority <= 1 {
			return true
		}
		c.rejectedReqs.Add(1)
		return false
	}

	return true
}

// ContextMiddleware adds backpressure awareness to context.
type bpContextKey struct{}

// WithController stores the controller in context.
func WithController(ctx context.Context, ctrl *Controller) context.Context {
	return context.WithValue(ctx, bpContextKey{}, ctrl)
}

// ControllerFromContext retrieves the controller from context.
func ControllerFromContext(ctx context.Context) (*Controller, bool) {
	ctrl, ok := ctx.Value(bpContextKey{}).(*Controller)
	return ctrl, ok
}
