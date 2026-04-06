package probe

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Status represents the outcome of a probe check.
type Status string

const (
	StatusOK       Status = "ok"
	StatusDegraded Status = "degraded"
	StatusFailed   Status = "failed"
)

// Check is a function that performs a health check.
type Check struct {
	Name     string
	Fn       func(ctx context.Context) error
	Timeout  time.Duration
	Critical bool // if true, failure makes the probe fail
}

// Result represents the outcome of running a check.
type Result struct {
	Name     string        `json:"name"`
	Status   Status        `json:"status"`
	Error    string        `json:"error,omitempty"`
	Duration time.Duration `json:"duration_ns"`
}

// ProbeResult is the combined result of all checks for a probe.
type ProbeResult struct {
	Status  Status   `json:"status"`
	Checks  []Result `json:"checks"`
	CheckAt string   `json:"checked_at"`
}

// Manager manages liveness, readiness, and startup probes for Kubernetes.
type Manager struct {
	mu             sync.RWMutex
	livenessChecks []Check
	readyChecks    []Check
	startupChecks  []Check

	ready   atomic.Bool
	started atomic.Bool
}

// NewManager creates a new probe manager.
func NewManager() *Manager {
	return &Manager{}
}

// AddLiveness adds a liveness check.
func (m *Manager) AddLiveness(check Check) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.livenessChecks = append(m.livenessChecks, check)
}

// AddReadiness adds a readiness check.
func (m *Manager) AddReadiness(check Check) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readyChecks = append(m.readyChecks, check)
}

// AddStartup adds a startup check.
func (m *Manager) AddStartup(check Check) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startupChecks = append(m.startupChecks, check)
}

// MarkReady marks the service as ready to serve traffic.
func (m *Manager) MarkReady() { m.ready.Store(true) }

// MarkNotReady marks the service as not ready.
func (m *Manager) MarkNotReady() { m.ready.Store(false) }

// MarkStarted marks the service as having completed startup.
func (m *Manager) MarkStarted() { m.started.Store(true) }

// IsReady returns whether the service is marked ready.
func (m *Manager) IsReady() bool { return m.ready.Load() }

// IsStarted returns whether startup has completed.
func (m *Manager) IsStarted() bool { return m.started.Load() }

// RunChecks runs a set of checks and returns the combined result.
func RunChecks(ctx context.Context, checks []Check) ProbeResult {
	results := make([]Result, len(checks))
	overall := StatusOK

	var wg sync.WaitGroup
	for i, check := range checks {
		wg.Add(1)
		go func(idx int, c Check) {
			defer wg.Done()

			timeout := c.Timeout
			if timeout == 0 {
				timeout = 5 * time.Second
			}

			checkCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			start := time.Now()
			err := c.Fn(checkCtx)
			duration := time.Since(start)

			r := Result{
				Name:     c.Name,
				Status:   StatusOK,
				Duration: duration,
			}

			if err != nil {
				r.Error = err.Error()
				if c.Critical {
					r.Status = StatusFailed
				} else {
					r.Status = StatusDegraded
				}
			}

			results[idx] = r
		}(i, check)
	}
	wg.Wait()

	for _, r := range results {
		if r.Status == StatusFailed {
			overall = StatusFailed
		} else if r.Status == StatusDegraded && overall != StatusFailed {
			overall = StatusDegraded
		}
	}

	return ProbeResult{
		Status:  overall,
		Checks:  results,
		CheckAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// LivenessHandler returns an HTTP handler for the /healthz endpoint.
func (m *Manager) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m.mu.RLock()
		checks := make([]Check, len(m.livenessChecks))
		copy(checks, m.livenessChecks)
		m.mu.RUnlock()

		result := RunChecks(r.Context(), checks)
		writeProbeResult(w, result)
	}
}

// ReadinessHandler returns an HTTP handler for the /readyz endpoint.
func (m *Manager) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !m.ready.Load() {
			writeProbeResult(w, ProbeResult{
				Status:  StatusFailed,
				CheckAt: time.Now().UTC().Format(time.RFC3339),
			})
			return
		}

		m.mu.RLock()
		checks := make([]Check, len(m.readyChecks))
		copy(checks, m.readyChecks)
		m.mu.RUnlock()

		result := RunChecks(r.Context(), checks)
		writeProbeResult(w, result)
	}
}

// StartupHandler returns an HTTP handler for the /startupz endpoint.
func (m *Manager) StartupHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !m.started.Load() {
			writeProbeResult(w, ProbeResult{
				Status:  StatusFailed,
				CheckAt: time.Now().UTC().Format(time.RFC3339),
			})
			return
		}

		m.mu.RLock()
		checks := make([]Check, len(m.startupChecks))
		copy(checks, m.startupChecks)
		m.mu.RUnlock()

		result := RunChecks(r.Context(), checks)
		writeProbeResult(w, result)
	}
}

// RegisterHandlers registers all probe handlers on the given mux.
func (m *Manager) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", m.LivenessHandler())
	mux.HandleFunc("GET /readyz", m.ReadinessHandler())
	mux.HandleFunc("GET /startupz", m.StartupHandler())
}

func writeProbeResult(w http.ResponseWriter, result ProbeResult) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")

	status := http.StatusOK
	if result.Status == StatusFailed {
		status = http.StatusServiceUnavailable
	}
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(result)
}
