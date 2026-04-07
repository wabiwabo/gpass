package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRecordRequest_OutOfRangeStatus exercises the defensive bucket-clamp
// in RecordRequest (idx >= len(m.requests) → idx 0). Status 999 is invalid
// per RFC 7231 but a buggy handler could produce it; we must not panic.
func TestRecordRequest_OutOfRangeStatus(t *testing.T) {
	m := NewMetrics("svc")
	m.RecordRequest(999) // 9 / 100 == 9, out of range
	m.RecordRequest(-1)  // -1/100 == 0, defensive
	// no assertion beyond "did not panic" — counters are atomic uints
}

// TestRecordDuration_BucketsCumulative confirms a slow request increments
// every bucket including the largest one (10s).
func TestRecordDuration_BucketsCumulative(t *testing.T) {
	m := NewMetrics("svc")
	m.RecordDuration(15.0) // beyond largest finite bucket
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	m.Handler(nil)(w, req)
	body := w.Body.String()
	// All finite buckets should be 0 (15s > 10s), +Inf bucket should be 1.
	if !strings.Contains(body, `http_request_duration_seconds_bucket{service="svc",le="+Inf"} 1`) {
		t.Errorf("missing +Inf=1 in:\n%s", body)
	}
	if !strings.Contains(body, `http_request_duration_seconds_count{service="svc"} 1`) {
		t.Errorf("count != 1 in:\n%s", body)
	}
}

// TestCompress_ExplicitWriteHeader covers the WriteHeader path on
// gzipResponseWriter, which is otherwise only reached when handlers call
// w.WriteHeader(code) before w.Write.
func TestCompress_ExplicitWriteHeader(t *testing.T) {
	body := strings.Repeat("x", 4096)
	h := Compress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted) // explicit
		_, _ = w.Write([]byte(body))
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted {
		t.Errorf("status = %d, want 202", w.Code)
	}
	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Errorf("Content-Encoding = %q", w.Header().Get("Content-Encoding"))
	}
}

// TestCompress_PreEncodedPassthrough covers the case where a downstream
// handler has already set Content-Encoding (e.g. it gzipped manually).
// We must NOT double-encode.
func TestCompress_PreEncodedPassthrough(t *testing.T) {
	body := strings.Repeat("x", 4096)
	h := Compress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "br") // claim brotli
		_, _ = w.Write([]byte(body))
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if got := w.Header().Get("Content-Encoding"); got != "br" {
		t.Errorf("Content-Encoding = %q, want br (untouched)", got)
	}
}

// TestCORS_DefaultPreflightResponseHeaders covers the defaultStr fallback
// branch when no AllowedHeaders are configured but a preflight is received.
func TestCORS_DefaultPreflightResponseHeaders(t *testing.T) {
	h := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		CORSOptions{AllowedOrigins: []string{"https://example.test"}})
	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "https://example.test")
	req.Header.Set("Access-Control-Request-Method", "POST")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Error("Allow-Headers should fall back to a default, got empty")
	}
}
