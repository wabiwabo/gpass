// Package healthcheck provides a composable health check system
// with support for readiness/liveness probes, dependency checks,
// and degraded state reporting. Follows Kubernetes health check patterns.
package healthcheck

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Status represents the health status.
type Status string

const (
	StatusUp       Status = "up"
	StatusDown     Status = "down"
	StatusDegraded Status = "degraded"
)

// Check is a function that checks a dependency's health.
type Check func(ctx context.Context) error

// Component represents a health check for a single dependency.
type Component struct {
	Name    string `json:"name"`
	Status  Status `json:"status"`
	Error   string `json:"error,omitempty"`
	Latency string `json:"latency,omitempty"`
}

// Report is the full health check response.
type Report struct {
	Status     Status      `json:"status"`
	Components []Component `json:"components,omitempty"`
	Version    string      `json:"version,omitempty"`
	Uptime     string      `json:"uptime,omitempty"`
}

// Checker manages health checks.
type Checker struct {
	mu         sync.RWMutex
	checks     map[string]checkEntry
	version    string
	startTime  time.Time
	timeout    time.Duration
}

type checkEntry struct {
	fn       Check
	critical bool // if true, failure = down; if false, failure = degraded
}

// Option configures a Checker.
type Option func(*Checker)

// WithVersion sets the service version in reports.
func WithVersion(v string) Option {
	return func(c *Checker) { c.version = v }
}

// WithTimeout sets the per-check timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Checker) { c.timeout = d }
}

// NewChecker creates a health checker.
func NewChecker(opts ...Option) *Checker {
	c := &Checker{
		checks:    make(map[string]checkEntry),
		startTime: time.Now(),
		timeout:   5 * time.Second,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// AddCheck registers a health check.
// Critical checks cause overall status "down" on failure.
// Non-critical checks cause "degraded" on failure.
func (c *Checker) AddCheck(name string, check Check, critical bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks[name] = checkEntry{fn: check, critical: critical}
}

// Run executes all health checks and returns a report.
func (c *Checker) Run(ctx context.Context) Report {
	c.mu.RLock()
	checks := make(map[string]checkEntry, len(c.checks))
	for k, v := range c.checks {
		checks[k] = v
	}
	c.mu.RUnlock()

	type result struct {
		name      string
		component Component
		critical  bool
	}

	results := make(chan result, len(checks))

	for name, entry := range checks {
		go func(name string, entry checkEntry) {
			checkCtx, cancel := context.WithTimeout(ctx, c.timeout)
			defer cancel()

			start := time.Now()
			err := entry.fn(checkCtx)
			latency := time.Since(start)

			comp := Component{
				Name:    name,
				Status:  StatusUp,
				Latency: latency.String(),
			}
			if err != nil {
				comp.Status = StatusDown
				comp.Error = err.Error()
			}

			results <- result{name: name, component: comp, critical: entry.critical}
		}(name, entry)
	}

	report := Report{
		Status:  StatusUp,
		Version: c.version,
		Uptime:  time.Since(c.startTime).Truncate(time.Second).String(),
	}

	for i := 0; i < len(checks); i++ {
		r := <-results
		report.Components = append(report.Components, r.component)

		if r.component.Status == StatusDown {
			if r.critical {
				report.Status = StatusDown
			} else if report.Status == StatusUp {
				report.Status = StatusDegraded
			}
		}
	}

	return report
}

// LivenessHandler returns an HTTP handler for liveness probes.
// Always returns 200 if the process is running.
func LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "up"})
	}
}

// ReadinessHandler returns an HTTP handler for readiness probes.
func (c *Checker) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report := c.Run(r.Context())

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")

		switch report.Status {
		case StatusUp:
			w.WriteHeader(http.StatusOK)
		case StatusDegraded:
			w.WriteHeader(http.StatusOK) // degraded but still serving
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		json.NewEncoder(w).Encode(report)
	}
}

// Count returns the number of registered checks.
func (c *Checker) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.checks)
}
