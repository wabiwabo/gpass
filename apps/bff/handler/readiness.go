package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ReadinessCheck defines a single dependency check.
type ReadinessCheck struct {
	Name     string
	Critical bool
	Check    func(ctx context.Context) error
}

// ReadinessResult is the JSON response for the readiness endpoint.
type ReadinessResult struct {
	Ready    bool              `json:"ready"`
	Checks   map[string]string `json:"checks"`
	Duration string            `json:"duration"`
}

// ReadinessHandler performs deep readiness checks for Kubernetes.
type ReadinessHandler struct {
	checks []ReadinessCheck
}

// NewReadinessHandler creates a readiness handler with dependency checks.
func NewReadinessHandler(checks ...ReadinessCheck) *ReadinessHandler {
	return &ReadinessHandler{checks: checks}
}

// ServeHTTP handles GET /ready.
// Runs all checks concurrently with a 5s timeout.
// Returns 200 if all critical checks pass, 503 otherwise.
func (h *ReadinessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	results := make(map[string]string, len(h.checks))
	var mu sync.Mutex
	var wg sync.WaitGroup

	criticalFailed := false

	for _, check := range h.checks {
		wg.Add(1)
		go func(c ReadinessCheck) {
			defer wg.Done()

			err := c.Check(ctx)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				results[c.Name] = fmt.Sprintf("error: %v", err)
				if c.Critical {
					criticalFailed = true
				}
			} else {
				results[c.Name] = "ok"
			}
		}(check)
	}

	wg.Wait()

	result := ReadinessResult{
		Ready:    !criticalFailed,
		Checks:   results,
		Duration: time.Since(start).String(),
	}

	status := http.StatusOK
	if criticalFailed {
		status = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(result)
}
