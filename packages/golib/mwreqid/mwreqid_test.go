package mwreqid

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGenerate(t *testing.T) {
	id := Generate()
	if len(id) != 16 {
		t.Errorf("length: got %d, want 16", len(id))
	}
	// Should be unique
	id2 := Generate()
	if id == id2 {
		t.Error("IDs should be unique")
	}
}

func TestMiddlewareGeneratesID(t *testing.T) {
	var gotID string
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = FromRequest(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if gotID == "" {
		t.Error("request ID should be generated")
	}
	if rr.Header().Get(HeaderName) == "" {
		t.Error("response should have X-Request-ID header")
	}
	if rr.Header().Get(HeaderName) != gotID {
		t.Error("response header should match context ID")
	}
}

func TestMiddlewarePreservesExisting(t *testing.T) {
	var gotID string
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = FromRequest(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(HeaderName, "existing-id-123")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if gotID != "existing-id-123" {
		t.Errorf("should preserve existing ID: got %q", gotID)
	}
	if rr.Header().Get(HeaderName) != "existing-id-123" {
		t.Error("response header should match")
	}
}

func TestFromContextEmpty(t *testing.T) {
	ctx := context.Background()
	if id := FromContext(ctx); id != "" {
		t.Errorf("should be empty, got %q", id)
	}
}

func TestFromContextWithValue(t *testing.T) {
	ctx := context.WithValue(context.Background(), reqIDKey, "test-id")
	if id := FromContext(ctx); id != "test-id" {
		t.Errorf("got %q, want %q", id, "test-id")
	}
}

func TestMiddlewareChain(t *testing.T) {
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "yes")
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("status: got %d", rr.Code)
	}
	if rr.Header().Get("X-Custom") != "yes" {
		t.Error("custom header should be preserved")
	}
}
