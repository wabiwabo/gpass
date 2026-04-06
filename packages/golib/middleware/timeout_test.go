package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTimeout_HandlerCompletes(t *testing.T) {
	handler := Timeout(5 * time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "ok" {
		t.Errorf("expected 'ok', got %q", w.Body.String())
	}
}

func TestTimeout_HandlerExceedsTimeout(t *testing.T) {
	handler := Timeout(50 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(200 * time.Millisecond):
			w.WriteHeader(http.StatusOK)
		case <-r.Context().Done():
			return
		}
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestTimeout_ContextCancelledInHandler(t *testing.T) {
	errCh := make(chan error, 1)
	handler := Timeout(50 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		errCh <- r.Context().Err()
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	select {
	case err := <-errCh:
		if err == nil {
			t.Error("expected context error after timeout")
		}
	case <-time.After(time.Second):
		t.Error("timed out waiting for context error")
	}
}
