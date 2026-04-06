package accesslog

import (
	"encoding/json"
	"net/http"
	"sort"
	"sync"
	"time"
)

const maxSamples = 1000

// EndpointStats holds per-endpoint latency and error statistics.
type EndpointStats struct {
	Path          string        `json:"path"`
	Method        string        `json:"method"`
	TotalRequests int64         `json:"total_requests"`
	ErrorCount    int64         `json:"error_count"`
	P50           time.Duration `json:"p50"`
	P95           time.Duration `json:"p95"`
	P99           time.Duration `json:"p99"`
	AvgLatency    time.Duration `json:"avg_latency"`
	LastRequest   time.Time     `json:"last_request"`
}

type endpointKey struct {
	method string
	path   string
}

type endpointData struct {
	totalRequests int64
	errorCount    int64
	latencies     []time.Duration
	totalLatency  time.Duration
	lastRequest   time.Time
}

// Recorder tracks per-endpoint latency and error statistics.
type Recorder struct {
	mu        sync.RWMutex
	endpoints map[endpointKey]*endpointData
}

// NewRecorder creates a new Recorder.
func NewRecorder() *Recorder {
	return &Recorder{
		endpoints: make(map[endpointKey]*endpointData),
	}
}

type responseWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.wrote {
		rw.status = code
		rw.wrote = true
	}
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wrote {
		rw.status = http.StatusOK
		rw.wrote = true
	}
	return rw.ResponseWriter.Write(b)
}

// Middleware returns an HTTP middleware that records request latency and status codes.
func (r *Recorder) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			start := time.Now()
			rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rw, req)
			elapsed := time.Since(start)

			r.record(req.Method, req.URL.Path, rw.status, elapsed)
		})
	}
}

func (r *Recorder) record(method, path string, status int, latency time.Duration) {
	key := endpointKey{method: method, path: path}
	now := time.Now()

	r.mu.Lock()
	defer r.mu.Unlock()

	ep, ok := r.endpoints[key]
	if !ok {
		ep = &endpointData{
			latencies: make([]time.Duration, 0, maxSamples),
		}
		r.endpoints[key] = ep
	}

	ep.totalRequests++
	if status >= 400 {
		ep.errorCount++
	}
	ep.totalLatency += latency
	ep.lastRequest = now

	// Rolling window: keep only last maxSamples latencies.
	if len(ep.latencies) >= maxSamples {
		copy(ep.latencies, ep.latencies[1:])
		ep.latencies = ep.latencies[:maxSamples-1]
	}
	ep.latencies = append(ep.latencies, latency)
}

// Stats returns statistics for all tracked endpoints.
func (r *Recorder) Stats() []EndpointStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := make([]EndpointStats, 0, len(r.endpoints))
	for key, ep := range r.endpoints {
		stats = append(stats, r.buildStats(key, ep))
	}

	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Method != stats[j].Method {
			return stats[i].Method < stats[j].Method
		}
		return stats[i].Path < stats[j].Path
	})

	return stats
}

// StatsForEndpoint returns statistics for a specific method and path.
func (r *Recorder) StatsForEndpoint(method, path string) (EndpointStats, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := endpointKey{method: method, path: path}
	ep, ok := r.endpoints[key]
	if !ok {
		return EndpointStats{}, false
	}
	return r.buildStats(key, ep), true
}

// Reset clears all recorded statistics.
func (r *Recorder) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.endpoints = make(map[endpointKey]*endpointData)
}

// Handler returns an http.HandlerFunc that serves all stats as JSON.
func (r *Recorder) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		stats := r.Stats()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	}
}

func (r *Recorder) buildStats(key endpointKey, ep *endpointData) EndpointStats {
	var avg time.Duration
	if ep.totalRequests > 0 {
		avg = time.Duration(int64(ep.totalLatency) / ep.totalRequests)
	}

	sorted := make([]time.Duration, len(ep.latencies))
	copy(sorted, ep.latencies)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	return EndpointStats{
		Path:          key.path,
		Method:        key.method,
		TotalRequests: ep.totalRequests,
		ErrorCount:    ep.errorCount,
		P50:           percentile(sorted, 0.50),
		P95:           percentile(sorted, 0.95),
		P99:           percentile(sorted, 0.99),
		AvgLatency:    avg,
		LastRequest:   ep.lastRequest,
	}
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}
