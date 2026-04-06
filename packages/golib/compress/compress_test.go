package compress

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMiddleware_CompressesLargeResponse(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinSize = 100

	handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(strings.Repeat(`{"key":"value"},`, 200)))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Error("should set Content-Encoding: gzip")
	}

	// Verify we can decompress.
	gr, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	body, _ := io.ReadAll(gr)
	gr.Close()

	if len(body) == 0 {
		t.Error("decompressed body should not be empty")
	}
}

func TestMiddleware_SkipsSmallResponse(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinSize = 1000

	handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") == "gzip" {
		t.Error("small response should not be compressed")
	}
}

func TestMiddleware_NoAcceptEncoding(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinSize = 10

	handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.Repeat("x", 1000)))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No Accept-Encoding header.
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") != "" {
		t.Error("should not compress without Accept-Encoding")
	}
}

func TestMiddleware_SkipPaths(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinSize = 10
	cfg.SkipPaths = map[string]bool{"/healthz": true}

	handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.Repeat("x", 2000)))
	}))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") == "gzip" {
		t.Error("skipped path should not be compressed")
	}
}

func TestNegotiateEncoding(t *testing.T) {
	tests := []struct {
		accept string
		want   string
	}{
		{"gzip, deflate", "gzip"},
		{"deflate", "deflate"},
		{"br, gzip", "gzip"},
		{"identity", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := negotiateEncoding(tt.accept)
		if got != tt.want {
			t.Errorf("negotiate(%q) = %q, want %q", tt.accept, got, tt.want)
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MinSize != 1024 {
		t.Errorf("min size: got %d", cfg.MinSize)
	}
	if cfg.Level != 6 {
		t.Errorf("level: got %d", cfg.Level)
	}
	if len(cfg.ContentTypes) == 0 {
		t.Error("should have default content types")
	}
}
