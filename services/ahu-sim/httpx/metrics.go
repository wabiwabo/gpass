package httpx

import (
	"database/sql"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics collects per-service request counters and exposes them in
// Prometheus text format. Stdlib-only — no client_golang dependency.
//
// Counters are bucketed by status class (2xx/3xx/4xx/5xx) instead of
// per-route to bound cardinality. Per-route metrics are an anti-pattern
// at the application layer; route them via API gateway labels instead.
// durationBuckets are the classic SLO histogram buckets in seconds.
// Mirrors Prometheus client_golang DefBuckets (5ms..10s).
var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

type Metrics struct {
	service string

	mu       sync.Mutex
	requests [6]uint64 // index by status class: 0=other, 1=1xx, 2=2xx, 3=3xx, 4=4xx, 5=5xx
	panics   uint64

	// Histogram of request durations (atomic counters per bucket).
	// Bucket index i counts requests with duration <= durationBuckets[i].
	// The +Inf bucket is implicit: total = sum of all requests.
	histBuckets [11]uint64 // len(durationBuckets)
	histSum     uint64     // microseconds (avoid float races)
	histCount   uint64     // total observations (== +Inf bucket)
}

// NewMetrics creates a metrics collector for the named service.
func NewMetrics(service string) *Metrics {
	return &Metrics{service: service}
}

// RecordRequest increments the counter for a status code's class.
func (m *Metrics) RecordRequest(status int) {
	idx := status / 100
	if idx < 0 || idx >= len(m.requests) {
		idx = 0
	}
	atomic.AddUint64(&m.requests[idx], 1)
}

// RecordPanic increments the panic counter (called by Recover middleware
// when wired through MetricsRecover).
func (m *Metrics) RecordPanic() {
	atomic.AddUint64(&m.panics, 1)
}

// RecordDuration adds an observation to the duration histogram.
// Cumulative bucket semantics: a 50ms request increments every bucket >= 0.05.
func (m *Metrics) RecordDuration(seconds float64) {
	for i, ub := range durationBuckets {
		if seconds <= ub {
			atomic.AddUint64(&m.histBuckets[i], 1)
		}
	}
	atomic.AddUint64(&m.histSum, uint64(seconds*1_000_000))
	atomic.AddUint64(&m.histCount, 1)
}

// Handler returns an http.HandlerFunc that emits Prometheus text format.
// Optionally pass a *sql.DB to include connection pool gauges.
func (m *Metrics) Handler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		fmt.Fprintf(w, "# HELP http_requests_total Total HTTP requests by status class\n")
		fmt.Fprintf(w, "# TYPE http_requests_total counter\n")
		for class, count := range m.requests {
			if class == 0 {
				continue
			}
			fmt.Fprintf(w, "http_requests_total{service=%q,class=\"%dxx\"} %d\n",
				m.service, class, atomic.LoadUint64(&m.requests[class]))
			_ = count
		}

		// Histogram: cumulative buckets + sum + count
		fmt.Fprintf(w, "# HELP http_request_duration_seconds Request duration histogram\n")
		fmt.Fprintf(w, "# TYPE http_request_duration_seconds histogram\n")
		for i, ub := range durationBuckets {
			c := atomic.LoadUint64(&m.histBuckets[i])
			fmt.Fprintf(w, "http_request_duration_seconds_bucket{service=%q,le=\"%g\"} %d\n",
				m.service, ub, c)
		}
		histCount := atomic.LoadUint64(&m.histCount)
		fmt.Fprintf(w, "http_request_duration_seconds_bucket{service=%q,le=\"+Inf\"} %d\n",
			m.service, histCount)
		fmt.Fprintf(w, "http_request_duration_seconds_sum{service=%q} %f\n",
			m.service, float64(atomic.LoadUint64(&m.histSum))/1_000_000)
		fmt.Fprintf(w, "http_request_duration_seconds_count{service=%q} %d\n",
			m.service, histCount)

		fmt.Fprintf(w, "# HELP http_panics_total Total panics recovered in HTTP handlers\n")
		fmt.Fprintf(w, "# TYPE http_panics_total counter\n")
		fmt.Fprintf(w, "http_panics_total{service=%q} %d\n",
			m.service, atomic.LoadUint64(&m.panics))

		// Go runtime gauges
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)
		fmt.Fprintf(w, "# HELP go_goroutines Current goroutine count\n")
		fmt.Fprintf(w, "# TYPE go_goroutines gauge\n")
		fmt.Fprintf(w, "go_goroutines{service=%q} %d\n", m.service, runtime.NumGoroutine())
		fmt.Fprintf(w, "# HELP go_memstats_alloc_bytes Heap bytes allocated\n")
		fmt.Fprintf(w, "# TYPE go_memstats_alloc_bytes gauge\n")
		fmt.Fprintf(w, "go_memstats_alloc_bytes{service=%q} %d\n", m.service, mem.Alloc)

		// DB pool gauges (if Postgres-backed)
		if db != nil {
			s := db.Stats()
			fmt.Fprintf(w, "# HELP db_pool_open_connections Currently open DB connections\n")
			fmt.Fprintf(w, "# TYPE db_pool_open_connections gauge\n")
			fmt.Fprintf(w, "db_pool_open_connections{service=%q} %d\n", m.service, s.OpenConnections)
			fmt.Fprintf(w, "# HELP db_pool_in_use Currently in-use DB connections\n")
			fmt.Fprintf(w, "# TYPE db_pool_in_use gauge\n")
			fmt.Fprintf(w, "db_pool_in_use{service=%q} %d\n", m.service, s.InUse)
			fmt.Fprintf(w, "# HELP db_pool_idle Currently idle DB connections\n")
			fmt.Fprintf(w, "# TYPE db_pool_idle gauge\n")
			fmt.Fprintf(w, "db_pool_idle{service=%q} %d\n", m.service, s.Idle)
			fmt.Fprintf(w, "# HELP db_pool_wait_count Total connections waited for\n")
			fmt.Fprintf(w, "# TYPE db_pool_wait_count counter\n")
			fmt.Fprintf(w, "db_pool_wait_count{service=%q} %d\n", m.service, s.WaitCount)
		}
	}
}

// Instrument wraps h so that completed requests increment metrics counters.
// Wire this OUTSIDE Recover so it sees the recovered status code, not 0.
func (m *Metrics) Instrument(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip self-metrics endpoint to avoid feedback loops
		if r.URL.Path == "/metrics" {
			h.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		h.ServeHTTP(rec, r)
		if rec.status == 0 {
			rec.status = http.StatusOK
		}
		m.RecordRequest(rec.status)
		m.RecordDuration(time.Since(start).Seconds())
	})
}

// Verify both methods used (so go vet doesn't flag the field as dead).
var _ = (&Metrics{}).RecordPanic
