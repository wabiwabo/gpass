package mwgzip

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
	cfg.MinSize = 10 // low threshold for testing
	mw := Middleware(cfg)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(strings.Repeat(`{"data":"test"}`, 100)))
	}))

	req := httptest.NewRequest("GET", "/api", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Error("should be gzip encoded")
	}

	// Verify can decompress
	reader, err := gzip.NewReader(w.Body)
	if err != nil { t.Fatalf("gzip reader: %v", err) }
	defer reader.Close()
	body, _ := io.ReadAll(reader)
	if !strings.Contains(string(body), "data") {
		t.Error("decompressed body wrong")
	}
}

func TestMiddleware_NoCompressSmall(t *testing.T) {
	mw := Middleware(DefaultConfig()) // 1024 min
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))

	req := httptest.NewRequest("GET", "/api", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") == "gzip" {
		t.Error("small response should not be compressed")
	}
}

func TestMiddleware_NoAcceptEncoding(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinSize = 1
	mw := Middleware(cfg)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.Repeat("x", 2000)))
	}))

	req := httptest.NewRequest("GET", "/api", nil)
	// No Accept-Encoding header
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") == "gzip" {
		t.Error("should not compress without Accept-Encoding")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MinSize != 1024 { t.Errorf("MinSize = %d", cfg.MinSize) }
	if len(cfg.ContentTypes) == 0 { t.Error("should have content types") }
}
