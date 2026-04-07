package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetrics_RecordAndRender(t *testing.T) {
	m := NewMetrics("test-svc")
	m.RecordRequest(200)
	m.RecordRequest(201)
	m.RecordRequest(404)
	m.RecordRequest(500)
	m.RecordPanic()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	m.Handler(nil)(w, req)

	body := w.Body.String()
	for _, want := range []string{
		`http_requests_total{service="test-svc",class="2xx"} 2`,
		`http_requests_total{service="test-svc",class="4xx"} 1`,
		`http_requests_total{service="test-svc",class="5xx"} 1`,
		`http_panics_total{service="test-svc"} 1`,
		`go_goroutines{service="test-svc"}`,
		`go_memstats_alloc_bytes{service="test-svc"}`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in:\n%s", want, body)
		}
	}
}

func TestMetrics_Instrument(t *testing.T) {
	m := NewMetrics("svc")
	h := m.Instrument(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot) // 418 -> 4xx
	}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if got := m.requests[4]; got != 1 {
		t.Errorf("4xx counter = %d, want 1", got)
	}
}

func TestMetrics_DurationHistogram(t *testing.T) {
	m := NewMetrics("svc")
	m.RecordDuration(0.001) // 1ms — falls in 5ms bucket and all above
	m.RecordDuration(0.7)   // 700ms — falls in 1s bucket and above
	m.RecordDuration(15)    // 15s — falls in no bucket but increments count

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	m.Handler(nil)(w, req)
	body := w.Body.String()

	for _, want := range []string{
		`http_request_duration_seconds_bucket{service="svc",le="0.005"} 1`,
		`http_request_duration_seconds_bucket{service="svc",le="1"} 2`,
		`http_request_duration_seconds_bucket{service="svc",le="10"} 2`,
		`http_request_duration_seconds_bucket{service="svc",le="+Inf"} 3`,
		`http_request_duration_seconds_count{service="svc"} 3`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in:\n%s", want, body)
		}
	}
}

func TestMetrics_InstrumentSkipsSelf(t *testing.T) {
	m := NewMetrics("svc")
	h := m.Instrument(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if got := m.requests[2]; got != 0 {
		t.Errorf("/metrics should not increment counters, got %d", got)
	}
}
