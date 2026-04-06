package mwpanic

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMiddlewareRecoversPanic(t *testing.T) {
	handler := Simple(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "internal_server_error") {
		t.Errorf("body should contain error code: %s", body)
	}
}

func TestMiddlewareNoPanic(t *testing.T) {
	handler := Simple(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rr.Code)
	}
	if rr.Body.String() != "ok" {
		t.Errorf("body: got %q", rr.Body.String())
	}
}

func TestMiddlewareCustomHandler(t *testing.T) {
	var recovered interface{}
	handler := Middleware(func(w http.ResponseWriter, r *http.Request, err interface{}) {
		recovered = err
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("custom"))
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("custom panic")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("status: got %d, want 503", rr.Code)
	}
	if rr.Body.String() != "custom" {
		t.Errorf("body: got %q", rr.Body.String())
	}
	if recovered != "custom panic" {
		t.Errorf("recovered: got %v", recovered)
	}
}

func TestMiddlewareNilHandler(t *testing.T) {
	handler := Middleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("nil handler panic")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rr.Code)
	}
}

func TestMiddlewarePanicWithError(t *testing.T) {
	handler := Simple(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rr.Code)
	}
}

func TestDefaultHandlerSetsJSON(t *testing.T) {
	handler := Simple(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("json check")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: got %q", ct)
	}
}

func TestMiddlewareChain(t *testing.T) {
	mw := Middleware(nil)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "passed")
		w.WriteHeader(http.StatusOK)
	})

	handler := mw(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("X-Test") != "passed" {
		t.Error("inner handler headers should be preserved")
	}
}
