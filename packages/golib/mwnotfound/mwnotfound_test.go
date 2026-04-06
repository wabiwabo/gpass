package mwnotfound

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJSON(t *testing.T) {
	handler := JSON()
	req := httptest.NewRequest("GET", "/missing", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: got %q", ct)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "not_found") {
		t.Errorf("body should contain error code: %s", body)
	}
	if !strings.Contains(body, "/missing") {
		t.Errorf("body should contain path: %s", body)
	}
}

func TestHandler(t *testing.T) {
	handler := Handler("custom message")
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "custom message") {
		t.Errorf("body should contain custom message: %s", rr.Body.String())
	}
}

func TestMiddlewarePassthrough(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/ok", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("found"))
	})

	handler := Middleware(mux)
	req := httptest.NewRequest("GET", "/api/ok", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rr.Code)
	}
	if rr.Body.String() != "found" {
		t.Errorf("body: got %q", rr.Body.String())
	}
}

func TestJSONContentType(t *testing.T) {
	handler := JSON()
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type: got %q", rr.Header().Get("Content-Type"))
	}
}
