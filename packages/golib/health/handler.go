package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Check represents a named health check.
type Check struct {
	Name string
	Fn   func(ctx context.Context) error
}

type response struct {
	Status  string            `json:"status"`
	Service string            `json:"service"`
	Checks  map[string]string `json:"checks"`
	Version string            `json:"version,omitempty"`
}

// Handler returns an http.HandlerFunc that runs all health checks concurrently
// with a 5-second timeout and returns a structured JSON response.
//
// Status: "ok" if all pass, "degraded" if some fail, "unhealthy" if all fail.
func Handler(service string, checks ...Check) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		results := make(map[string]string, len(checks))
		var mu sync.Mutex
		var wg sync.WaitGroup

		for _, c := range checks {
			wg.Add(1)
			go func(c Check) {
				defer wg.Done()
				err := c.Fn(ctx)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					results[c.Name] = err.Error()
				} else {
					results[c.Name] = "ok"
				}
			}(c)
		}

		wg.Wait()

		passed := 0
		for _, v := range results {
			if v == "ok" {
				passed++
			}
		}

		status := "ok"
		if len(checks) > 0 {
			if passed == 0 {
				status = "unhealthy"
			} else if passed < len(checks) {
				status = "degraded"
			}
		}

		resp := response{
			Status:  status,
			Service: service,
			Checks:  results,
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if status == "unhealthy" {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		json.NewEncoder(w).Encode(resp)
	}
}
