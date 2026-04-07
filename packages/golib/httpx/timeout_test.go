package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTimeout_DeadlinePropagated(t *testing.T) {
	h := Timeout(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dl, ok := r.Context().Deadline()
		if !ok {
			t.Error("expected deadline on context")
		}
		if time.Until(dl) > 100*time.Millisecond {
			t.Errorf("deadline too far: %v", time.Until(dl))
		}
		w.WriteHeader(http.StatusOK)
	}), 50*time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestTimeout_HandlerSeesDeadlineExceeded(t *testing.T) {
	h := Timeout(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(100 * time.Millisecond):
			w.WriteHeader(http.StatusOK)
		case <-r.Context().Done():
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}), 10*time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
}

func TestTimeout_ZeroDefaults(t *testing.T) {
	called := false
	h := Timeout(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		dl, ok := r.Context().Deadline()
		if !ok || time.Until(dl) < 25*time.Second {
			t.Errorf("expected default ~30s deadline, got %v / ok=%v", time.Until(dl), ok)
		}
	}), 0)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(httptest.NewRecorder(), req)
	if !called {
		t.Error("handler not called")
	}
}
