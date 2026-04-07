package compress

import (
	"compress/flate"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestCompress_DeflateNegotiation covers the deflate branch of
// negotiateEncoding + the flate.NewWriter path in flush. The existing
// tests only exercise gzip.
func TestCompress_DeflateNegotiation(t *testing.T) {
	body := strings.Repeat("hello world ", 200) // 2400 bytes > 1024 minSize
	mw := Middleware(Config{})
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(body))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "deflate")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if got := w.Header().Get("Content-Encoding"); got != "deflate" {
		t.Errorf("Content-Encoding = %q, want deflate", got)
	}
	r := flate.NewReader(w.Body)
	defer r.Close()
	got, _ := io.ReadAll(r)
	if string(got) != body {
		t.Errorf("decompressed mismatch: %d vs %d bytes", len(got), len(body))
	}
}

// TestCompress_QualityValueAndMixedAccept covers the parser branches in
// negotiateEncoding when the client sends a quality-value list like
// browsers do (`gzip;q=1.0, deflate;q=0.5`).
func TestCompress_QualityValueAndMixedAccept(t *testing.T) {
	mw := Middleware(Config{})
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(strings.Repeat("x", 2048)))
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "br;q=1.0, gzip;q=0.9, deflate;q=0.5")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	// We don't speak br, so the next preferred is gzip.
	if got := w.Header().Get("Content-Encoding"); got != "gzip" {
		t.Errorf("Content-Encoding = %q, want gzip (br unsupported, fall through)", got)
	}
}

// TestCompress_SkipPaths covers the SkipPaths early-return branch.
func TestCompress_SkipPaths(t *testing.T) {
	mw := Middleware(Config{
		SkipPaths: map[string]bool{"/metrics": true},
	})
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 5000)))
	}))
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Header().Get("Content-Encoding") == "gzip" {
		t.Error("/metrics should be skipped — must not be gzipped")
	}
}

// TestCompress_ContentTypeAllowList covers the typeSet enforcement:
// content types not in the allow-list pass through uncompressed even
// when they exceed MinSize.
func TestCompress_ContentTypeAllowList(t *testing.T) {
	mw := Middleware(Config{
		ContentTypes: []string{"application/json"},
	})
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		_, _ = w.Write([]byte(strings.Repeat("a,b,c\n", 500)))
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Header().Get("Content-Encoding") == "gzip" {
		t.Error("text/csv not in allow-list — must not be gzipped")
	}
}

// TestCompress_ExplicitWriteHeaderFlushesBuffer covers the WriteHeader
// branch in compressWriter, previously 0%. A handler that calls
// WriteHeader explicitly before any Write must flush whatever's in the
// buffer (typically empty here) and forward the status.
func TestCompress_ExplicitWriteHeaderBeforeWrite(t *testing.T) {
	mw := Middleware(Config{})
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(strings.Repeat("x", 2048)))
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted {
		t.Errorf("status = %d, want 202", w.Code)
	}
}

// TestCompress_SmallPayloadPassthrough covers the "below MinSize" branch:
// the buffer never reaches the threshold, so flush() must write the raw
// buffered data without setting Content-Encoding.
func TestCompress_SmallPayloadPassthrough(t *testing.T) {
	mw := Middleware(Config{MinSize: 1024})
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("tiny"))
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Header().Get("Content-Encoding") == "gzip" {
		t.Error("4-byte body should not be gzipped (below 1024 MinSize)")
	}
	if w.Body.String() != "tiny" {
		t.Errorf("body = %q", w.Body.String())
	}
}

// TestCompress_MultiWriteAcrossThreshold pins the path where Write is
// called multiple times and the running buffer crosses MinSize on the
// second call: first chunk buffers, second chunk triggers flush, then
// subsequent writes go through the gzip writer.
func TestCompress_MultiWriteAcrossThreshold(t *testing.T) {
	mw := Middleware(Config{MinSize: 100})
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(strings.Repeat("a", 50)))  // under threshold
		_, _ = w.Write([]byte(strings.Repeat("b", 100))) // crosses
		_, _ = w.Write([]byte(strings.Repeat("c", 200))) // direct via gzip
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("not gzipped: headers=%v", w.Header())
	}
	gz, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	got, _ := io.ReadAll(gz)
	want := strings.Repeat("a", 50) + strings.Repeat("b", 100) + strings.Repeat("c", 200)
	if string(got) != want {
		t.Errorf("decompressed mismatch: %d vs %d", len(got), len(want))
	}
}
