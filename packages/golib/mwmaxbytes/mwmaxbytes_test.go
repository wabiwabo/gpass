package mwmaxbytes

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxBytes != 1<<20 {
		t.Errorf("MaxBytes: got %d, want %d", cfg.MaxBytes, 1<<20)
	}
	if cfg.Message == "" {
		t.Error("Message should not be empty")
	}
}

func TestMiddlewareRejectsLargeContentLength(t *testing.T) {
	handler := Middleware(Config{MaxBytes: 100})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", 200)))
	req.ContentLength = 200
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusRequestEntityTooLarge)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "payload_too_large") {
		t.Errorf("body should contain error code: %s", body)
	}
	if !strings.Contains(body, "100") {
		t.Errorf("body should contain max_bytes: %s", body)
	}
}

func TestMiddlewareAllowsSmallRequest(t *testing.T) {
	var bodyRead bool
	handler := Middleware(Config{MaxBytes: 1024})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
			return
		}
		if string(data) != "hello" {
			t.Errorf("body: got %q, want %q", data, "hello")
		}
		bodyRead = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("hello"))
	req.ContentLength = 5
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}
	if !bodyRead {
		t.Error("handler should have been called")
	}
}

func TestMiddlewareDefaultsZeroMaxBytes(t *testing.T) {
	handler := Middleware(Config{MaxBytes: 0})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("small"))
	req.ContentLength = 5
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestMiddlewareDefaultsNegativeMaxBytes(t *testing.T) {
	handler := Middleware(Config{MaxBytes: -1})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("test"))
	req.ContentLength = 4
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestMiddlewareDefaultsEmptyMessage(t *testing.T) {
	handler := Middleware(Config{MaxBytes: 10, Message: ""})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", 100)))
	req.ContentLength = 100
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusRequestEntityTooLarge)
	}
	if !strings.Contains(rr.Body.String(), "request body too large") {
		t.Error("should use default message")
	}
}

func TestMiddlewareCustomMessage(t *testing.T) {
	handler := Middleware(Config{MaxBytes: 10, Message: "too big!"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", 100)))
	req.ContentLength = 100
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !strings.Contains(rr.Body.String(), "too big!") {
		t.Errorf("should use custom message: %s", rr.Body.String())
	}
}

func TestMiddlewareMaxBytesReaderEnforced(t *testing.T) {
	// Even if Content-Length is not set, MaxBytesReader limits actual reading
	handler := Middleware(Config{MaxBytes: 10})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", 100)))
	req.ContentLength = -1 // Unknown content length
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestMiddlewareGETRequest(t *testing.T) {
	handler := Middleware(Config{MaxBytes: 10})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("GET should pass through, got status %d", rr.Code)
	}
}

func TestMiddlewareExactLimit(t *testing.T) {
	handler := Middleware(Config{MaxBytes: 5})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "error", http.StatusInternalServerError)
			return
		}
		if string(data) != "12345" {
			t.Errorf("body: got %q, want %q", data, "12345")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("12345"))
	req.ContentLength = 5
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("exact limit should pass, got status %d", rr.Code)
	}
}

func TestSimple(t *testing.T) {
	var called bool
	handler := Simple(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("small"))
	req.ContentLength = 5
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("handler should have been called")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestSimpleRejectsLarge(t *testing.T) {
	handler := Simple(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("x"))
	req.ContentLength = 2 << 20 // 2MB
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestMiddlewareResponseJSON(t *testing.T) {
	handler := Middleware(Config{MaxBytes: 10})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", 100)))
	req.ContentLength = 100
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, `"error"`) {
		t.Errorf("response should be JSON-like: %s", body)
	}
	if !strings.Contains(body, `"payload_too_large"`) {
		t.Errorf("response should contain error code: %s", body)
	}
	if !strings.Contains(body, `"max_bytes"`) {
		t.Errorf("response should contain max_bytes: %s", body)
	}
}

func TestMiddlewareChaining(t *testing.T) {
	mw := Middleware(Config{MaxBytes: 1024})
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "yes")
		w.WriteHeader(http.StatusOK)
	})

	handler := mw(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("X-Custom") != "yes" {
		t.Error("inner handler headers should be preserved")
	}
}
