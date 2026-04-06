package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRecord_IncrementsRequestCounter(t *testing.T) {
	m := New("test-svc")
	m.Record("GET", "/health", 200, 10*time.Millisecond)
	m.Record("GET", "/health", 200, 20*time.Millisecond)

	m.mu.RLock()
	defer m.mu.RUnlock()

	c, ok := m.requests["GET:/health:200"]
	if !ok {
		t.Fatal("expected counter for GET:/health:200")
	}
	if c.value != 2 {
		t.Errorf("expected 2, got %d", c.value)
	}
}

func TestRecord_UpdatesDurationHistogram(t *testing.T) {
	m := New("test-svc")
	m.Record("GET", "/api", 200, 5*time.Millisecond)  // 0.005s, fits in 0.005 bucket
	m.Record("GET", "/api", 200, 50*time.Millisecond) // 0.05s, fits in 0.05 bucket and above

	m.mu.RLock()
	defer m.mu.RUnlock()

	h, ok := m.durations["GET:/api"]
	if !ok {
		t.Fatal("expected histogram for GET:/api")
	}
	if h.count != 2 {
		t.Errorf("expected count 2, got %d", h.count)
	}
	if h.sum < 0.050 {
		t.Errorf("expected sum >= 0.050, got %f", h.sum)
	}
	// The 0.005 bucket should have count 1 (only the 5ms request)
	if h.buckets[0.005] != 1 {
		t.Errorf("expected bucket[0.005] = 1, got %d", h.buckets[0.005])
	}
	// The 0.05 bucket should have count 2 (both requests fit in le=0.05)
	if h.buckets[0.05] != 2 {
		t.Errorf("expected bucket[0.05] = 2, got %d", h.buckets[0.05])
	}
}

func TestMiddleware_RecordsAutomatically(t *testing.T) {
	m := New("bff")
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.requests["GET:/health:200"]; !ok {
		t.Error("expected request to be recorded")
	}
	if _, ok := m.durations["GET:/health"]; !ok {
		t.Error("expected duration to be recorded")
	}
}

func TestHandler_OutputsValidPrometheusFormat(t *testing.T) {
	m := New("bff")
	m.Record("GET", "/health", 200, 3*time.Millisecond)

	r := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	m.Handler().ServeHTTP(w, r)

	body := w.Body.String()

	// Check content type
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Errorf("expected text/plain content type, got %q", ct)
	}

	// Check counter
	if !strings.Contains(body, "# TYPE http_requests_total counter") {
		t.Error("missing counter TYPE declaration")
	}
	if !strings.Contains(body, `http_requests_total{service="bff",method="GET",path="/health",status="200"} 1`) {
		t.Errorf("missing or wrong counter line, body:\n%s", body)
	}

	// Check histogram
	if !strings.Contains(body, "# TYPE http_request_duration_seconds histogram") {
		t.Error("missing histogram TYPE declaration")
	}
	if !strings.Contains(body, `http_request_duration_seconds_bucket{service="bff",method="GET",path="/health",le="+Inf"} 1`) {
		t.Errorf("missing +Inf bucket, body:\n%s", body)
	}
	if !strings.Contains(body, `http_request_duration_seconds_count{service="bff",method="GET",path="/health"} 1`) {
		t.Errorf("missing count line, body:\n%s", body)
	}

	// Check gauge
	if !strings.Contains(body, "# TYPE http_requests_in_flight gauge") {
		t.Error("missing gauge TYPE declaration")
	}
	if !strings.Contains(body, `http_requests_in_flight{service="bff"}`) {
		t.Errorf("missing in-flight gauge, body:\n%s", body)
	}
}

func TestInFlight_IncrementsAndDecrements(t *testing.T) {
	m := New("bff")

	started := make(chan struct{})
	release := make(chan struct{})

	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusOK)
	}))

	go func() {
		r := httptest.NewRequest(http.MethodGet, "/slow", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
	}()

	<-started
	if got := m.InFlight(); got != 1 {
		t.Errorf("expected in-flight 1, got %d", got)
	}

	close(release)
	// Wait for goroutine to finish
	time.Sleep(50 * time.Millisecond)

	if got := m.InFlight(); got != 0 {
		t.Errorf("expected in-flight 0, got %d", got)
	}
}

func TestMultiplePathsTrackedSeparately(t *testing.T) {
	m := New("bff")
	m.Record("GET", "/health", 200, time.Millisecond)
	m.Record("POST", "/api/login", 200, 10*time.Millisecond)
	m.Record("GET", "/api/users", 200, 5*time.Millisecond)

	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.requests) != 3 {
		t.Errorf("expected 3 request entries, got %d", len(m.requests))
	}
	if len(m.durations) != 3 {
		t.Errorf("expected 3 duration entries, got %d", len(m.durations))
	}
}

func TestStatusCodesTrackedCorrectly(t *testing.T) {
	m := New("bff")
	m.Record("GET", "/api", 200, time.Millisecond)
	m.Record("GET", "/api", 200, time.Millisecond)
	m.Record("GET", "/api", 404, time.Millisecond)
	m.Record("GET", "/api", 500, time.Millisecond)

	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.requests["GET:/api:200"].value != 2 {
		t.Errorf("expected 2 for 200, got %d", m.requests["GET:/api:200"].value)
	}
	if m.requests["GET:/api:404"].value != 1 {
		t.Errorf("expected 1 for 404, got %d", m.requests["GET:/api:404"].value)
	}
	if m.requests["GET:/api:500"].value != 1 {
		t.Errorf("expected 1 for 500, got %d", m.requests["GET:/api:500"].value)
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := New("bff")
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Record("GET", "/health", 200, time.Millisecond)
		}()
	}
	wg.Wait()

	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.requests["GET:/health:200"].value != 100 {
		t.Errorf("expected 100, got %d", m.requests["GET:/health:200"].value)
	}
}
