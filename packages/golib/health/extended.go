package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// ExtendedHealth implements a comprehensive health check response
// following the IETF Health Check Response Format draft (RFC draft-inadarei-api-health-check).
type ExtendedHealth struct {
	Status      string                   `json:"status"`
	Version     string                   `json:"version"`
	ReleaseID   string                   `json:"releaseId,omitempty"`
	Description string                   `json:"description"`
	ServiceID   string                   `json:"serviceId"`
	Checks      map[string][]CheckResult `json:"checks"`
	Links       map[string]string        `json:"links,omitempty"`
	Output      string                   `json:"output,omitempty"`
}

// CheckResult represents the result of a single health check component.
type CheckResult struct {
	ComponentID   string      `json:"componentId,omitempty"`
	ComponentType string      `json:"componentType,omitempty"`
	Status        string      `json:"status"`
	ObservedValue interface{} `json:"observedValue,omitempty"`
	ObservedUnit  string      `json:"observedUnit,omitempty"`
	Time          string      `json:"time"`
	Output        string      `json:"output,omitempty"`
}

// ExtendedHandler creates a health handler following the IETF Health Check Response Format.
// Checks are run concurrently with a 5-second timeout.
// The overall status is determined by the worst individual check status:
//   - all pass -> "pass"
//   - any warn (none fail) -> "warn"
//   - any fail -> "fail"
func ExtendedHandler(serviceName, version, description string, checks map[string]func(ctx context.Context) CheckResult) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		results := make(map[string][]CheckResult, len(checks))
		var mu sync.Mutex
		var wg sync.WaitGroup

		for name, fn := range checks {
			wg.Add(1)
			go func(name string, fn func(ctx context.Context) CheckResult) {
				defer wg.Done()

				done := make(chan CheckResult, 1)
				go func() {
					done <- fn(ctx)
				}()

				var cr CheckResult
				select {
				case cr = <-done:
				case <-ctx.Done():
					cr = CheckResult{
						Status: "fail",
						Time:   time.Now().UTC().Format(time.RFC3339),
						Output: "check timed out",
					}
				}

				mu.Lock()
				results[name] = []CheckResult{cr}
				mu.Unlock()
			}(name, fn)
		}

		wg.Wait()

		overallStatus := "pass"
		for _, crs := range results {
			for _, cr := range crs {
				switch cr.Status {
				case "fail":
					overallStatus = "fail"
				case "warn":
					if overallStatus != "fail" {
						overallStatus = "warn"
					}
				}
			}
		}

		resp := ExtendedHealth{
			Status:      overallStatus,
			Version:     version,
			Description: description,
			ServiceID:   serviceName,
			Checks:      results,
		}

		w.Header().Set("Content-Type", "application/health+json")
		switch overallStatus {
		case "fail":
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			w.WriteHeader(http.StatusOK)
		}
		json.NewEncoder(w).Encode(resp)
	}
}
