// Package healthagg provides health aggregation across multiple services.
// It collects health status from registered endpoints and produces a
// unified health report with overall status and per-service detail.
package healthagg

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"
)

// Status represents overall health status.
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusDegraded  Status = "degraded"
	StatusUnhealthy Status = "unhealthy"
)

// ServiceHealth holds health info for one service.
type ServiceHealth struct {
	Name     string        `json:"name"`
	URL      string        `json:"url"`
	Status   Status        `json:"status"`
	Latency  time.Duration `json:"latency"`
	Error    string        `json:"error,omitempty"`
	Critical bool          `json:"critical"`
	LastCheck time.Time    `json:"last_check"`
}

// Report is the aggregated health report.
type Report struct {
	Status    Status          `json:"status"`
	Services  []ServiceHealth `json:"services"`
	Timestamp time.Time       `json:"timestamp"`
	Duration  time.Duration   `json:"duration"`
	Healthy   int             `json:"healthy"`
	Degraded  int             `json:"degraded"`
	Unhealthy int             `json:"unhealthy"`
}

// Service defines a service to health-check.
type Service struct {
	Name     string
	URL      string        // Health check URL (must return 200 for healthy).
	Timeout  time.Duration // Per-service timeout.
	Critical bool          // If true, failure marks overall as unhealthy.
}

// Aggregator collects and reports health status across services.
type Aggregator struct {
	mu       sync.RWMutex
	services []Service
	client   *http.Client
	cache    *Report
	cacheTTL time.Duration
}

// NewAggregator creates a health aggregator.
func NewAggregator() *Aggregator {
	return &Aggregator{
		client: &http.Client{Timeout: 5 * time.Second},
		cacheTTL: 10 * time.Second,
	}
}

// SetCacheTTL sets how long health reports are cached.
func (a *Aggregator) SetCacheTTL(d time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cacheTTL = d
}

// Add registers a service for health checking.
func (a *Aggregator) Add(svc Service) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if svc.Timeout <= 0 {
		svc.Timeout = 3 * time.Second
	}
	a.services = append(a.services, svc)
}

// Check runs health checks on all registered services concurrently.
func (a *Aggregator) Check(ctx context.Context) Report {
	// Check cache.
	a.mu.RLock()
	if a.cache != nil && time.Since(a.cache.Timestamp) < a.cacheTTL {
		report := *a.cache
		a.mu.RUnlock()
		return report
	}
	a.mu.RUnlock()

	start := time.Now()

	a.mu.RLock()
	services := make([]Service, len(a.services))
	copy(services, a.services)
	a.mu.RUnlock()

	results := make([]ServiceHealth, len(services))
	var wg sync.WaitGroup

	for i, svc := range services {
		wg.Add(1)
		go func(idx int, s Service) {
			defer wg.Done()
			results[idx] = a.checkService(ctx, s)
		}(i, svc)
	}
	wg.Wait()

	// Sort by name for deterministic output.
	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	report := a.buildReport(results, time.Since(start))

	// Update cache.
	a.mu.Lock()
	a.cache = &report
	a.mu.Unlock()

	return report
}

func (a *Aggregator) checkService(ctx context.Context, svc Service) ServiceHealth {
	sh := ServiceHealth{
		Name:      svc.Name,
		URL:       svc.URL,
		Critical:  svc.Critical,
		LastCheck: time.Now(),
	}

	checkCtx, cancel := context.WithTimeout(ctx, svc.Timeout)
	defer cancel()

	start := time.Now()

	req, err := http.NewRequestWithContext(checkCtx, http.MethodGet, svc.URL, nil)
	if err != nil {
		sh.Status = StatusUnhealthy
		sh.Error = fmt.Sprintf("create request: %v", err)
		sh.Latency = time.Since(start)
		return sh
	}

	resp, err := a.client.Do(req)
	sh.Latency = time.Since(start)

	if err != nil {
		sh.Status = StatusUnhealthy
		sh.Error = err.Error()
		return sh
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		sh.Status = StatusHealthy
	} else if resp.StatusCode == http.StatusServiceUnavailable {
		sh.Status = StatusUnhealthy
		sh.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
	} else {
		sh.Status = StatusDegraded
		sh.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	return sh
}

func (a *Aggregator) buildReport(services []ServiceHealth, duration time.Duration) Report {
	report := Report{
		Status:    StatusHealthy,
		Services:  services,
		Timestamp: time.Now(),
		Duration:  duration,
	}

	for _, svc := range services {
		switch svc.Status {
		case StatusHealthy:
			report.Healthy++
		case StatusDegraded:
			report.Degraded++
			if report.Status == StatusHealthy {
				report.Status = StatusDegraded
			}
		case StatusUnhealthy:
			report.Unhealthy++
			if svc.Critical {
				report.Status = StatusUnhealthy
			} else if report.Status == StatusHealthy {
				report.Status = StatusDegraded
			}
		}
	}

	return report
}

// Handler returns an HTTP handler that serves the health report.
func (a *Aggregator) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report := a.Check(r.Context())

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")

		switch report.Status {
		case StatusHealthy:
			w.WriteHeader(http.StatusOK)
		case StatusDegraded:
			w.WriteHeader(http.StatusOK)
		case StatusUnhealthy:
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		json.NewEncoder(w).Encode(report)
	}
}

// Count returns the number of registered services.
func (a *Aggregator) Count() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.services)
}

// InvalidateCache clears the cached health report.
func (a *Aggregator) InvalidateCache() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cache = nil
}
