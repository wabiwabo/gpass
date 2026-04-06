package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestID_GeneratesNew(t *testing.T) {
	var ctxID string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxID = GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	respID := w.Header().Get("X-Request-Id")
	if respID == "" {
		t.Error("expected X-Request-Id in response header")
	}
	if ctxID == "" {
		t.Error("expected request ID in context")
	}
	if respID != ctxID {
		t.Errorf("response header %q != context value %q", respID, ctxID)
	}
}

func TestRequestID_PreservesExisting(t *testing.T) {
	existingID := "my-custom-id-123"

	var ctxID string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxID = GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Request-Id", existingID)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if respID := w.Header().Get("X-Request-Id"); respID != existingID {
		t.Errorf("expected %q, got %q", existingID, respID)
	}
	if ctxID != existingID {
		t.Errorf("expected context ID %q, got %q", existingID, ctxID)
	}
}

func TestRequestID_AvailableInContext(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := GetRequestID(r.Context())
		if id == "" {
			t.Error("expected non-empty request ID in context")
		}
		w.Write([]byte(id))
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Body.Len() == 0 {
		t.Error("expected body to contain request ID")
	}
}

func TestGetRequestID_EmptyContext(t *testing.T) {
	if id := GetRequestID(nil); id != "" {
		t.Errorf("expected empty string for nil context, got %q", id)
	}
}
