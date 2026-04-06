package maxbody

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMiddleware_AllowsSmallBody(t *testing.T) {
	handler := Middleware(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	body := bytes.NewBufferString(`{"name":"test"}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Length", "15")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("small body: got %d", w.Code)
	}
}

func TestMiddleware_RejectsLargeBody(t *testing.T) {
	handler := Middleware(100)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not be called")
	}))

	body := bytes.NewBufferString(strings.Repeat("x", 200))
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.ContentLength = 200
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("large body: got %d, want 413", w.Code)
	}
}

func TestMiddleware_DefaultLimit(t *testing.T) {
	handler := Middleware(0)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	body := bytes.NewBufferString(`{"ok":true}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("default limit: got %d", w.Code)
	}
}

func TestPerRoute_GET(t *testing.T) {
	handler := PerRoute(0, 1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET: got %d", w.Code)
	}
}

func TestPerRoute_POST_Large(t *testing.T) {
	handler := PerRoute(0, 100)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not be called")
	}))

	body := bytes.NewBufferString(strings.Repeat("x", 200))
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.ContentLength = 200
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("POST large: got %d", w.Code)
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		got := FormatSize(tt.bytes)
		if got != tt.want {
			t.Errorf("FormatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}
