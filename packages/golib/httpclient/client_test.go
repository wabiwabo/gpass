package httpclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/garudapass/gpass/packages/golib/circuitbreaker"
)

type testPayload struct {
	Name string `json:"name"`
}

func TestPostSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}
		if accept := r.Header.Get("Accept"); accept != "application/json" {
			t.Errorf("expected Accept application/json, got %s", accept)
		}

		var req testPayload
		json.NewDecoder(r.Body).Decode(&req)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testPayload{Name: "response-" + req.Name})
	}))
	defer srv.Close()

	c := New(srv.URL, WithTimeout(5*time.Second))

	var result testPayload
	err := c.Post(context.Background(), "/test", testPayload{Name: "alice"}, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "response-alice" {
		t.Errorf("expected 'response-alice', got %q", result.Name)
	}
}

func TestGetSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testPayload{Name: "bob"})
	}))
	defer srv.Close()

	c := New(srv.URL)

	var result testPayload
	err := c.Get(context.Background(), "/data", &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "bob" {
		t.Errorf("expected 'bob', got %q", result.Name)
	}
}

func TestWithAPIKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if key := r.Header.Get("X-API-Key"); key != "secret-key" {
			t.Errorf("expected X-API-Key 'secret-key', got %q", key)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testPayload{Name: "ok"})
	}))
	defer srv.Close()

	c := New(srv.URL, WithAPIKey("secret-key"))
	var result testPayload
	err := c.Get(context.Background(), "/", &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServerErrorTriggersCircuitBreaker(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"fail"}`))
	}))
	defer srv.Close()

	cb := circuitbreaker.New(2, time.Second)
	c := New(srv.URL, WithCircuitBreaker(cb))

	// Two failures should trip the breaker
	c.Get(context.Background(), "/", nil)
	c.Get(context.Background(), "/", nil)

	if cb.State() != circuitbreaker.StateOpen {
		t.Errorf("expected circuit breaker open, got %s", cb.State())
	}
}

func TestCircuitBreakerBlocksAfterThreshold(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cb := circuitbreaker.New(2, time.Minute)
	c := New(srv.URL, WithCircuitBreaker(cb))

	c.Get(context.Background(), "/", nil)
	c.Get(context.Background(), "/", nil)

	// Third call should be blocked by circuit breaker
	err := c.Get(context.Background(), "/", nil)
	if err == nil {
		t.Fatal("expected error from circuit breaker")
	}
	if calls != 2 {
		t.Errorf("expected 2 calls to server, got %d", calls)
	}
}
