package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecover_NoPanic(t *testing.T) {
	h := Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestRecover_NilDereferencePanic(t *testing.T) {
	h := Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p *int
		_ = *p // boom
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	// Should NOT panic out of ServeHTTP
	h.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestRecover_StringPanic(t *testing.T) {
	h := Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("kaboom")
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
	if w.Body.String() == "" {
		t.Error("expected error body")
	}
}
