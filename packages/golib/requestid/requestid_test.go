package requestid

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGenerate_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := Generate()
		if seen[id] {
			t.Fatal("duplicate ID generated")
		}
		seen[id] = true
	}
}

func TestGenerate_Length(t *testing.T) {
	id := Generate()
	if len(id) != 24 {
		t.Errorf("length: got %d, want 24", len(id))
	}
}

func TestContext_RoundTrip(t *testing.T) {
	ctx := ToContext(context.Background(), "req-123")
	got := FromContext(ctx)
	if got != "req-123" {
		t.Errorf("got %q", got)
	}
}

func TestFromContext_Missing(t *testing.T) {
	got := FromContext(context.Background())
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestFromRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-Id", "existing-id")
	if FromRequest(req) != "existing-id" {
		t.Error("should read from header")
	}
}

func TestMiddleware_GeneratesID(t *testing.T) {
	var gotID string
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if gotID == "" {
		t.Error("should generate request ID")
	}
	if w.Header().Get("X-Request-Id") == "" {
		t.Error("should set response header")
	}
	if w.Header().Get("X-Request-Id") != gotID {
		t.Error("response header should match context ID")
	}
}

func TestMiddleware_PreservesExisting(t *testing.T) {
	var gotID string
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = FromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-Id", "preserved-id")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if gotID != "preserved-id" {
		t.Errorf("should preserve existing ID: got %q", gotID)
	}
}

func TestSequentialGenerator(t *testing.T) {
	gen := NewSequentialGenerator("test")

	id1 := gen.Next()
	id2 := gen.Next()
	id3 := gen.Next()

	if id1 == id2 || id2 == id3 {
		t.Error("sequential IDs should be unique")
	}
	if id1[:5] != "test-" {
		t.Errorf("should have prefix: got %q", id1)
	}
}

func TestSequentialGenerator_Concurrent(t *testing.T) {
	gen := NewSequentialGenerator("c")
	seen := make(map[string]bool)
	ch := make(chan string, 100)

	for i := 0; i < 100; i++ {
		go func() { ch <- gen.Next() }()
	}

	for i := 0; i < 100; i++ {
		id := <-ch
		if seen[id] {
			t.Fatal("duplicate sequential ID")
		}
		seen[id] = true
	}
}
