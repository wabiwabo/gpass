package chaos

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"sync/atomic"
	"time"
)

// FaultInjector injects controlled failures into HTTP handlers.
type FaultInjector struct {
	enabled    atomic.Bool
	errorRate  atomic.Int32 // percentage 0-100
	latencyMs  atomic.Int32 // added latency in ms
	statusCode atomic.Int32 // error status code to return
}

// New creates a new FaultInjector (disabled by default, 500 status).
func New() *FaultInjector {
	f := &FaultInjector{}
	f.statusCode.Store(500)
	return f
}

// Enable enables fault injection.
func (f *FaultInjector) Enable() {
	f.enabled.Store(true)
}

// Disable disables fault injection.
func (f *FaultInjector) Disable() {
	f.enabled.Store(false)
}

// IsEnabled reports whether fault injection is enabled.
func (f *FaultInjector) IsEnabled() bool {
	return f.enabled.Load()
}

// SetErrorRate sets the percentage of requests that will fail (0-100).
func (f *FaultInjector) SetErrorRate(pct int) {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	f.errorRate.Store(int32(pct))
}

// SetLatency sets additional latency to inject (milliseconds).
func (f *FaultInjector) SetLatency(ms int) {
	if ms < 0 {
		ms = 0
	}
	f.latencyMs.Store(int32(ms))
}

// SetStatusCode sets the HTTP status code for injected errors (default 500).
func (f *FaultInjector) SetStatusCode(code int) {
	f.statusCode.Store(int32(code))
}

// Middleware returns HTTP middleware that injects faults when enabled.
func (f *FaultInjector) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !f.enabled.Load() {
			next.ServeHTTP(w, r)
			return
		}

		// Inject latency.
		if ms := f.latencyMs.Load(); ms > 0 {
			time.Sleep(time.Duration(ms) * time.Millisecond)
		}

		// Inject error based on error rate.
		rate := f.errorRate.Load()
		if rate > 0 && rand.Intn(100) < int(rate) {
			code := int(f.statusCode.Load())
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(code)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":   "chaos_fault",
				"message": "fault injected by chaos engineering",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Handler returns an HTTP handler for controlling fault injection at runtime.
//
//	POST /internal/chaos/enable   — enable fault injection
//	POST /internal/chaos/disable  — disable fault injection
//	POST /internal/chaos/config   — set error_rate, latency_ms, status_code
//	GET  /internal/chaos/status   — current configuration
func (f *FaultInjector) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /internal/chaos/enable", func(w http.ResponseWriter, r *http.Request) {
		f.Enable()
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"enabled": true,
		})
	})

	mux.HandleFunc("POST /internal/chaos/disable", func(w http.ResponseWriter, r *http.Request) {
		f.Disable()
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"enabled": false,
		})
	})

	mux.HandleFunc("POST /internal/chaos/config", func(w http.ResponseWriter, r *http.Request) {
		var cfg struct {
			ErrorRate  *int `json:"error_rate"`
			LatencyMs  *int `json:"latency_ms"`
			StatusCode *int `json:"status_code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid JSON body",
			})
			return
		}

		if cfg.ErrorRate != nil {
			f.SetErrorRate(*cfg.ErrorRate)
		}
		if cfg.LatencyMs != nil {
			f.SetLatency(*cfg.LatencyMs)
		}
		if cfg.StatusCode != nil {
			f.SetStatusCode(*cfg.StatusCode)
		}

		writeJSON(w, http.StatusOK, f.status())
	})

	mux.HandleFunc("GET /internal/chaos/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, f.status())
	})

	return mux
}

func (f *FaultInjector) status() map[string]interface{} {
	return map[string]interface{}{
		"enabled":     f.enabled.Load(),
		"error_rate":  f.errorRate.Load(),
		"latency_ms":  f.latencyMs.Load(),
		"status_code": f.statusCode.Load(),
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
