package depcheck

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Dependency represents an external dependency to check.
type Dependency struct {
	Name     string
	Type     string // "database", "cache", "service", "queue"
	Check    func(ctx context.Context) error
	Timeout  time.Duration
	Required bool // if true, failure makes overall status unhealthy
}

// Status represents the health status of a dependency.
type Status struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Healthy  bool   `json:"healthy"`
	Latency  string `json:"latency"`
	Error    string `json:"error,omitempty"`
	Required bool   `json:"required"`
}

// Report is the aggregated health report.
type Report struct {
	Status       string   `json:"status"` // "healthy", "degraded", "unhealthy"
	Dependencies []Status `json:"dependencies"`
	CheckedAt    string   `json:"checked_at"`
	Duration     string   `json:"duration"`
}

// Checker performs concurrent dependency health checks.
type Checker struct {
	mu   sync.RWMutex
	deps []Dependency
}

// NewChecker creates a new dependency checker.
func NewChecker() *Checker {
	return &Checker{}
}

// Add registers a dependency.
func (c *Checker) Add(dep Dependency) {
	if dep.Timeout == 0 {
		dep.Timeout = 5 * time.Second
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deps = append(c.deps, dep)
}

// Check runs all dependency checks concurrently and returns the report.
func (c *Checker) Check(ctx context.Context) Report {
	start := time.Now()

	c.mu.RLock()
	deps := make([]Dependency, len(c.deps))
	copy(deps, c.deps)
	c.mu.RUnlock()

	statuses := make([]Status, len(deps))
	var wg sync.WaitGroup

	for i, dep := range deps {
		wg.Add(1)
		go func(idx int, d Dependency) {
			defer wg.Done()

			checkCtx, cancel := context.WithTimeout(ctx, d.Timeout)
			defer cancel()

			checkStart := time.Now()
			err := d.Check(checkCtx)
			latency := time.Since(checkStart)

			s := Status{
				Name:     d.Name,
				Type:     d.Type,
				Healthy:  err == nil,
				Latency:  latency.String(),
				Required: d.Required,
			}
			if err != nil {
				s.Error = err.Error()
			}
			statuses[idx] = s
		}(i, dep)
	}
	wg.Wait()

	overall := "healthy"
	for _, s := range statuses {
		if !s.Healthy && s.Required {
			overall = "unhealthy"
			break
		}
		if !s.Healthy {
			if overall == "healthy" {
				overall = "degraded"
			}
		}
	}

	return Report{
		Status:       overall,
		Dependencies: statuses,
		CheckedAt:    time.Now().UTC().Format(time.RFC3339),
		Duration:     time.Since(start).String(),
	}
}

// Handler returns an HTTP handler that serves the health report.
func (c *Checker) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report := c.Check(r.Context())

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")

		status := http.StatusOK
		if report.Status == "unhealthy" {
			status = http.StatusServiceUnavailable
		}
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(report)
	}
}

// Count returns the number of registered dependencies.
func (c *Checker) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.deps)
}
