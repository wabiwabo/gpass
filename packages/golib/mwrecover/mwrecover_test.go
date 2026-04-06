package mwrecover

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMiddleware_NoPanic(t *testing.T) {
	mw := Middleware(DefaultConfig())
	var called bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called { t.Error("should call handler") }
	if w.Code != 200 { t.Errorf("status = %d", w.Code) }
}

func TestMiddleware_RecoversPanic(t *testing.T) {
	mw := Middleware(DefaultConfig())
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("something went wrong")
	}))

	req := httptest.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 500 { t.Errorf("status = %d, want 500", w.Code) }
	if w.Header().Get("Content-Type") != "application/json" {
		t.Error("should set JSON content type")
	}
	if !strings.Contains(w.Body.String(), "internal_error") {
		t.Errorf("body = %q", w.Body.String())
	}
}

func TestMiddleware_LogsPanic(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	mw := Middleware(Config{Logger: logger, LogStack: true})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	output := buf.String()
	if !strings.Contains(output, "panic recovered") {
		t.Error("should log panic")
	}
	if !strings.Contains(output, "test panic") {
		t.Error("should contain panic value")
	}
	if !strings.Contains(output, "goroutine") {
		t.Error("should contain stack trace")
	}
}

func TestMiddleware_NoStack(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	mw := Middleware(Config{Logger: logger, LogStack: false})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("no stack")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if strings.Contains(buf.String(), "goroutine") {
		t.Error("should not log stack when LogStack=false")
	}
}

func TestMiddleware_CustomBody(t *testing.T) {
	mw := Middleware(Config{ResponseBody: `{"custom":"error"}`})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("fail")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Body.String() != `{"custom":"error"}` {
		t.Errorf("body = %q", w.Body.String())
	}
}

func TestSimple(t *testing.T) {
	handler := Simple(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("simple panic")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 500 { t.Errorf("status = %d", w.Code) }
}
