package httpx

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCompress_LargePayloadGzipped(t *testing.T) {
	body := strings.Repeat("hello world ", 200) // 2400 bytes
	h := Compress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", w.Header().Get("Content-Encoding"))
	}
	if !strings.Contains(w.Header().Get("Vary"), "Accept-Encoding") {
		t.Errorf("Vary missing Accept-Encoding")
	}

	gz, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	got, _ := io.ReadAll(gz)
	if string(got) != body {
		t.Errorf("decompressed mismatch: %d bytes vs %d", len(got), len(body))
	}
}

func TestCompress_SmallPayloadPassthrough(t *testing.T) {
	body := "ok"
	h := Compress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") == "gzip" {
		t.Error("small payload should not be gzipped")
	}
	if w.Body.String() != body {
		t.Errorf("body = %q", w.Body.String())
	}
}

func TestCompress_NoAcceptEncoding(t *testing.T) {
	body := strings.Repeat("x", 5000)
	h := Compress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Header().Get("Content-Encoding") == "gzip" {
		t.Error("client did not request gzip")
	}
}

func TestCompress_PrecompressedSkipped(t *testing.T) {
	body := strings.Repeat("x", 5000)
	h := Compress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte(body))
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Header().Get("Content-Encoding") == "gzip" {
		t.Error("jpeg should not be re-compressed")
	}
}

func TestIsPrecompressed(t *testing.T) {
	for _, ct := range []string{"image/jpeg", "video/mp4", "application/zip", "image/PNG"} {
		if !isPrecompressed(ct) {
			t.Errorf("%s should be precompressed", ct)
		}
	}
	for _, ct := range []string{"application/json", "text/html", "text/plain"} {
		if isPrecompressed(ct) {
			t.Errorf("%s should NOT be precompressed", ct)
		}
	}
}
