package accesslog

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func makeHandler(status int, body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		w.Write([]byte(body))
	})
}

func makeSlowHandler(d time.Duration, status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(d)
		w.WriteHeader(status)
	})
}

func doRequest(handler http.Handler, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func TestRecorder_RecordsRequests(t *testing.T) {
	rec := NewRecorder()
	handler := rec.Middleware()(makeHandler(http.StatusOK, "ok"))

	doRequest(handler, http.MethodGet, "/api/test")
	doRequest(handler, http.MethodGet, "/api/test")
	doRequest(handler, http.MethodGet, "/api/test")

	stats := rec.Stats()
	if len(stats) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(stats))
	}
	if stats[0].TotalRequests != 3 {
		t.Errorf("expected 3 total requests, got %d", stats[0].TotalRequests)
	}
	if stats[0].Method != http.MethodGet {
		t.Errorf("expected method GET, got %s", stats[0].Method)
	}
	if stats[0].Path != "/api/test" {
		t.Errorf("expected path /api/test, got %s", stats[0].Path)
	}
}

func TestRecorder_TracksLatency(t *testing.T) {
	rec := NewRecorder()
	handler := rec.Middleware()(makeSlowHandler(10*time.Millisecond, http.StatusOK))

	for i := 0; i < 20; i++ {
		doRequest(handler, http.MethodGet, "/slow")
	}

	s, ok := rec.StatsForEndpoint(http.MethodGet, "/slow")
	if !ok {
		t.Fatal("endpoint not found")
	}

	if s.P50 <= 0 {
		t.Error("P50 should be > 0")
	}
	if s.P95 <= 0 {
		t.Error("P95 should be > 0")
	}
	if s.P99 <= 0 {
		t.Error("P99 should be > 0")
	}
	if s.AvgLatency <= 0 {
		t.Error("AvgLatency should be > 0")
	}
	if s.P50 > s.P95 {
		t.Error("P50 should be <= P95")
	}
	if s.P95 > s.P99 {
		t.Error("P95 should be <= P99")
	}
}

func TestRecorder_TracksErrors(t *testing.T) {
	rec := NewRecorder()

	okHandler := rec.Middleware()(makeHandler(http.StatusOK, "ok"))
	errHandler := rec.Middleware()(makeHandler(http.StatusInternalServerError, "err"))
	badHandler := rec.Middleware()(makeHandler(http.StatusBadRequest, "bad"))

	doRequest(okHandler, http.MethodPost, "/submit")
	doRequest(okHandler, http.MethodPost, "/submit")
	doRequest(errHandler, http.MethodPost, "/submit")
	doRequest(badHandler, http.MethodPost, "/submit")

	s, ok := rec.StatsForEndpoint(http.MethodPost, "/submit")
	if !ok {
		t.Fatal("endpoint not found")
	}
	if s.TotalRequests != 4 {
		t.Errorf("expected 4 requests, got %d", s.TotalRequests)
	}
	if s.ErrorCount != 2 {
		t.Errorf("expected 2 errors, got %d", s.ErrorCount)
	}
}

func TestRecorder_PerEndpoint(t *testing.T) {
	rec := NewRecorder()
	handler := rec.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	doRequest(handler, http.MethodGet, "/a")
	doRequest(handler, http.MethodGet, "/a")
	doRequest(handler, http.MethodPost, "/b")
	doRequest(handler, http.MethodGet, "/b")

	stats := rec.Stats()
	if len(stats) != 3 {
		t.Fatalf("expected 3 endpoint entries, got %d", len(stats))
	}

	sa, ok := rec.StatsForEndpoint(http.MethodGet, "/a")
	if !ok {
		t.Fatal("GET /a not found")
	}
	if sa.TotalRequests != 2 {
		t.Errorf("GET /a: expected 2 requests, got %d", sa.TotalRequests)
	}

	sb, ok := rec.StatsForEndpoint(http.MethodPost, "/b")
	if !ok {
		t.Fatal("POST /b not found")
	}
	if sb.TotalRequests != 1 {
		t.Errorf("POST /b: expected 1 request, got %d", sb.TotalRequests)
	}
}

func TestRecorder_StatsForEndpoint(t *testing.T) {
	rec := NewRecorder()
	handler := rec.Middleware()(makeHandler(http.StatusOK, "ok"))

	doRequest(handler, http.MethodGet, "/exists")

	if _, ok := rec.StatsForEndpoint(http.MethodGet, "/exists"); !ok {
		t.Error("expected to find GET /exists")
	}
	if _, ok := rec.StatsForEndpoint(http.MethodGet, "/missing"); ok {
		t.Error("expected GET /missing to be missing")
	}
	if _, ok := rec.StatsForEndpoint(http.MethodPost, "/exists"); ok {
		t.Error("expected POST /exists to be missing")
	}
}

func TestRecorder_Reset(t *testing.T) {
	rec := NewRecorder()
	handler := rec.Middleware()(makeHandler(http.StatusOK, "ok"))

	doRequest(handler, http.MethodGet, "/reset-test")

	if len(rec.Stats()) == 0 {
		t.Fatal("expected stats before reset")
	}

	rec.Reset()

	if len(rec.Stats()) != 0 {
		t.Error("expected no stats after reset")
	}
	if _, ok := rec.StatsForEndpoint(http.MethodGet, "/reset-test"); ok {
		t.Error("expected endpoint gone after reset")
	}
}

func TestRecorder_Handler_JSON(t *testing.T) {
	rec := NewRecorder()
	handler := rec.Middleware()(makeHandler(http.StatusOK, "ok"))

	doRequest(handler, http.MethodGet, "/json-test")
	doRequest(handler, http.MethodPost, "/json-test")

	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	rr := httptest.NewRecorder()
	rec.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}

	var result []EndpointStats
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 entries in JSON, got %d", len(result))
	}
}

func TestRecorder_ConcurrentAccess(t *testing.T) {
	rec := NewRecorder()
	handler := rec.Middleware()(makeHandler(http.StatusOK, "ok"))

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			doRequest(handler, http.MethodGet, "/concurrent")
			rec.Stats()
			rec.StatsForEndpoint(http.MethodGet, "/concurrent")
		}()
	}
	wg.Wait()

	s, ok := rec.StatsForEndpoint(http.MethodGet, "/concurrent")
	if !ok {
		t.Fatal("endpoint not found")
	}
	if s.TotalRequests != 100 {
		t.Errorf("expected 100 requests, got %d", s.TotalRequests)
	}
}

func TestRecorder_RollingWindow(t *testing.T) {
	rec := NewRecorder()
	handler := rec.Middleware()(makeHandler(http.StatusOK, "ok"))

	// Send more than maxSamples requests.
	for i := 0; i < maxSamples+500; i++ {
		doRequest(handler, http.MethodGet, "/rolling")
	}

	rec.mu.RLock()
	key := endpointKey{method: http.MethodGet, path: "/rolling"}
	ep := rec.endpoints[key]
	latencyCount := len(ep.latencies)
	rec.mu.RUnlock()

	if latencyCount > maxSamples {
		t.Errorf("latencies should not exceed %d, got %d", maxSamples, latencyCount)
	}
	if latencyCount != maxSamples {
		t.Errorf("expected exactly %d latencies, got %d", maxSamples, latencyCount)
	}

	s, _ := rec.StatsForEndpoint(http.MethodGet, "/rolling")
	if s.TotalRequests != int64(maxSamples+500) {
		t.Errorf("expected %d total requests, got %d", maxSamples+500, s.TotalRequests)
	}
}
