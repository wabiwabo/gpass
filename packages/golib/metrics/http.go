package metrics

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// HTTPMetrics tracks HTTP request metrics compatible with Prometheus exposition format.
type HTTPMetrics struct {
	serviceName string
	mu          sync.RWMutex
	requests    map[string]*counter   // key: method:path:status
	durations   map[string]*histogram // key: method:path
	inFlight    atomic.Int64
}

type counter struct {
	value int64
}

type histogram struct {
	count   int64
	sum     float64
	buckets map[float64]int64 // upper bound -> count
}

// DefaultBuckets for HTTP request duration (seconds).
var DefaultBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

// New creates HTTP metrics for a service.
func New(serviceName string) *HTTPMetrics {
	return &HTTPMetrics{
		serviceName: serviceName,
		requests:    make(map[string]*counter),
		durations:   make(map[string]*histogram),
	}
}

// Middleware returns HTTP middleware that records request metrics.
func (m *HTTPMetrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.inFlight.Add(1)
		defer m.inFlight.Add(-1)

		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		m.Record(r.Method, r.URL.Path, sw.status, time.Since(start))
	})
}

// Handler returns an HTTP handler that exposes metrics in Prometheus text format.
func (m *HTTPMetrics) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		var buf strings.Builder

		m.mu.RLock()
		defer m.mu.RUnlock()

		// http_requests_total counter
		buf.WriteString("# HELP http_requests_total Total HTTP requests\n")
		buf.WriteString("# TYPE http_requests_total counter\n")

		reqKeys := sortedKeys(m.requests)
		for _, key := range reqKeys {
			c := m.requests[key]
			method, path, status := parseRequestKey(key)
			fmt.Fprintf(&buf,
				"http_requests_total{service=%q,method=%q,path=%q,status=%q} %d\n",
				m.serviceName, method, path, status, c.value,
			)
		}

		// http_request_duration_seconds histogram
		buf.WriteString("\n# HELP http_request_duration_seconds HTTP request duration\n")
		buf.WriteString("# TYPE http_request_duration_seconds histogram\n")

		durKeys := sortedKeys(m.durations)
		for _, key := range durKeys {
			h := m.durations[key]
			method, path := parseDurationKey(key)

			bucketBounds := make([]float64, 0, len(h.buckets))
			for b := range h.buckets {
				bucketBounds = append(bucketBounds, b)
			}
			sort.Float64s(bucketBounds)

			for _, le := range bucketBounds {
				fmt.Fprintf(&buf,
					"http_request_duration_seconds_bucket{service=%q,method=%q,path=%q,le=%q} %d\n",
					m.serviceName, method, path, formatFloat(le), h.buckets[le],
				)
			}
			fmt.Fprintf(&buf,
				"http_request_duration_seconds_bucket{service=%q,method=%q,path=%q,le=\"+Inf\"} %d\n",
				m.serviceName, method, path, h.count,
			)
			fmt.Fprintf(&buf,
				"http_request_duration_seconds_sum{service=%q,method=%q,path=%q} %s\n",
				m.serviceName, method, path, formatFloat(h.sum),
			)
			fmt.Fprintf(&buf,
				"http_request_duration_seconds_count{service=%q,method=%q,path=%q} %d\n",
				m.serviceName, method, path, h.count,
			)
		}

		// http_requests_in_flight gauge
		buf.WriteString("\n# HELP http_requests_in_flight Current in-flight requests\n")
		buf.WriteString("# TYPE http_requests_in_flight gauge\n")
		fmt.Fprintf(&buf,
			"http_requests_in_flight{service=%q} %d\n",
			m.serviceName, m.inFlight.Load(),
		)

		w.Write([]byte(buf.String()))
	}
}

// Record records a completed request.
func (m *HTTPMetrics) Record(method, path string, statusCode int, duration time.Duration) {
	status := strconv.Itoa(statusCode)
	reqKey := method + ":" + path + ":" + status
	durKey := method + ":" + path
	seconds := duration.Seconds()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Increment request counter.
	c, ok := m.requests[reqKey]
	if !ok {
		c = &counter{}
		m.requests[reqKey] = c
	}
	c.value++

	// Update histogram.
	h, ok := m.durations[durKey]
	if !ok {
		h = &histogram{buckets: make(map[float64]int64, len(DefaultBuckets))}
		for _, b := range DefaultBuckets {
			h.buckets[b] = 0
		}
		m.durations[durKey] = h
	}
	h.count++
	h.sum += seconds
	for _, b := range DefaultBuckets {
		if seconds <= b {
			h.buckets[b]++
		}
	}
}

// InFlight returns current in-flight requests.
func (m *HTTPMetrics) InFlight() int64 {
	return m.inFlight.Load()
}

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (sw *statusWriter) WriteHeader(code int) {
	if !sw.wroteHeader {
		sw.status = code
		sw.wroteHeader = true
	}
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *statusWriter) Write(b []byte) (int, error) {
	if !sw.wroteHeader {
		sw.wroteHeader = true
	}
	return sw.ResponseWriter.Write(b)
}

func parseRequestKey(key string) (method, path, status string) {
	parts := strings.SplitN(key, ":", 3)
	return parts[0], parts[1], parts[2]
}

func parseDurationKey(key string) (method, path string) {
	parts := strings.SplitN(key, ":", 2)
	return parts[0], parts[1]
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
