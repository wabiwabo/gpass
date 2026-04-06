package mwxframe

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeny(t *testing.T) {
	handler := Deny(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("X-Frame-Options") != "DENY" {
		t.Errorf("got %q, want DENY", rr.Header().Get("X-Frame-Options"))
	}
}

func TestSameOrigin(t *testing.T) {
	handler := SameOrigin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("X-Frame-Options") != "SAMEORIGIN" {
		t.Errorf("got %q, want SAMEORIGIN", rr.Header().Get("X-Frame-Options"))
	}
}

func TestDenyPassthrough(t *testing.T) {
	var called bool
	handler := Deny(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("handler should be called")
	}
	if rr.Body.String() != "ok" {
		t.Errorf("body: got %q", rr.Body.String())
	}
}
