package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCompress_CompressesLargeResponse(t *testing.T) {
	body := strings.Repeat("hello world ", 200) // well above any minSize
	handler := Compress(100)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(body))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Fatal("expected Content-Encoding: gzip")
	}

	// Verify it's actually valid gzip and decompresses to original
	gr, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer gr.Close()
	decompressed, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("failed to decompress: %v", err)
	}
	if string(decompressed) != body {
		t.Errorf("decompressed output mismatch: got %d bytes, want %d bytes", len(decompressed), len(body))
	}
}

func TestCompress_NoCompressWithoutAcceptEncoding(t *testing.T) {
	body := strings.Repeat("data", 500)
	handler := Compress(100)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(body))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No Accept-Encoding header
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") == "gzip" {
		t.Fatal("should not compress without Accept-Encoding: gzip")
	}
	if w.Body.String() != body {
		t.Error("body should be uncompressed original")
	}
}

func TestCompress_NoCompressSmallResponse(t *testing.T) {
	body := "tiny"
	handler := Compress(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(body))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") == "gzip" {
		t.Fatal("should not compress responses below minSize")
	}
	if w.Body.String() != body {
		t.Error("body should be uncompressed original")
	}
}

func TestCompress_NoCompressImagePNG(t *testing.T) {
	body := strings.Repeat("\x89PNG fake image data ", 200)
	handler := Compress(100)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte(body))
	}))

	req := httptest.NewRequest(http.MethodGet, "/image.png", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") == "gzip" {
		t.Fatal("should not compress image/png content type")
	}
}

func TestCompress_SetsContentEncodingHeader(t *testing.T) {
	body := strings.Repeat("test content ", 200)
	handler := Compress(50)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Fatal("expected Content-Encoding: gzip header")
	}
}

func TestCompress_DecompressedMatchesOriginal(t *testing.T) {
	original := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 100)
	handler := Compress(100)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(original))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	gr, err := gzip.NewReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("gzip reader error: %v", err)
	}
	defer gr.Close()

	got, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("decompress error: %v", err)
	}

	if string(got) != original {
		t.Errorf("decompressed output does not match original\ngot length:  %d\nwant length: %d", len(got), len(original))
	}
}
