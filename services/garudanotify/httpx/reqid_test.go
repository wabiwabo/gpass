package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestID_Generated(t *testing.T) {
	var captured string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if captured == "" {
		t.Error("expected ID generated and propagated to context")
	}
	if got := w.Header().Get(HeaderRequestID); got != captured {
		t.Errorf("response header = %q, ctx = %q (should match)", got, captured)
	}
	if len(captured) != 32 {
		t.Errorf("generated ID length = %d, want 32 (hex of 16 bytes)", len(captured))
	}
}

func TestRequestID_Preserved(t *testing.T) {
	var captured string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderRequestID, "trace-abc-123")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if captured != "trace-abc-123" {
		t.Errorf("got %q, want preserved trace-abc-123", captured)
	}
}

func TestRequestID_OversizedRejected(t *testing.T) {
	var captured string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = RequestIDFromContext(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	big := make([]byte, 200)
	for i := range big {
		big[i] = 'x'
	}
	req.Header.Set(HeaderRequestID, string(big))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if len(captured) != 32 {
		t.Errorf("oversized ID should be rejected and regenerated; got len=%d", len(captured))
	}
}

func TestRequestIDFromContext_Missing(t *testing.T) {
	if got := RequestIDFromContext(httptest.NewRequest(http.MethodGet, "/", nil).Context()); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}
