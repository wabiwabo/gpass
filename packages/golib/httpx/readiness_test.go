package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadiness_OKInMemory(t *testing.T) {
	r := NewReadiness("svc", nil)
	w := httptest.NewRecorder()
	r.Handler()(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"db":"in-memory"`) {
		t.Errorf("body = %s", w.Body.String())
	}
}

func TestReadiness_DrainingReturns503(t *testing.T) {
	r := NewReadiness("svc", nil)
	r.Drain()
	if !r.IsDraining() {
		t.Error("IsDraining() = false after Drain()")
	}
	w := httptest.NewRecorder()
	r.Handler()(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"status":"draining"`) {
		t.Errorf("body = %s", w.Body.String())
	}
}

func TestReadiness_DrainIdempotent(t *testing.T) {
	r := NewReadiness("svc", nil)
	r.Drain()
	r.Drain()
	if !r.IsDraining() {
		t.Error("draining should still be true")
	}
}
