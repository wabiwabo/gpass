package oss

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestSearchByNIB_HappyPath pins the SearchByNIB success branch.
func TestSearchByNIB_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req NIBSearchRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.NIB != "1234567890123" {
			t.Errorf("NIB = %q", req.NIB)
		}
		json.NewEncoder(w).Encode(NIBSearchResponse{
			Found:  true,
			NIB:    "1234567890123",
			Name:   "PT Demo",
			Status: "ACTIVE",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key", 5*time.Second)
	resp, err := c.SearchByNIB(context.Background(), "1234567890123")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Name != "PT Demo" {
		t.Errorf("got %+v", resp)
	}
}

// TestDoPost_StatusError pins the non-200 branch.
func TestDoPost_StatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte("not found"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k", 5*time.Second)
	_, err := c.SearchByNPWP(context.Background(), "x")
	if err == nil || !strings.Contains(err.Error(), "status 404") {
		t.Errorf("err = %v", err)
	}
}

// TestDoPost_DialFailure pins the httpClient.Do error branch.
func TestDoPost_DialFailure(t *testing.T) {
	c := NewClient("http://127.0.0.1:1", "", 200*time.Millisecond)
	_, err := c.SearchByNPWP(context.Background(), "x")
	if err == nil || !strings.Contains(err.Error(), "OSS request failed") {
		t.Errorf("err = %v", err)
	}
}

// TestCircuitBreaker_OpensAfterThreshold pins the open transition.
func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "", 2*time.Second)
	for i := 0; i < 5; i++ {
		_, _ = c.SearchByNPWP(context.Background(), "x")
	}
	_, err := c.SearchByNPWP(context.Background(), "x")
	if err == nil || !strings.Contains(err.Error(), "circuit breaker open") {
		t.Errorf("breaker not open: %v", err)
	}
}

// TestCircuitBreaker_HalfOpenTransition pins the cooldown expiry.
func TestCircuitBreaker_HalfOpenTransition(t *testing.T) {
	cb := newCircuitBreaker(2, 20*time.Millisecond)
	cb.recordFailure()
	cb.recordFailure()
	if cb.allow() {
		t.Error("breaker should be open")
	}
	time.Sleep(40 * time.Millisecond)
	if !cb.allow() {
		t.Error("breaker should transition to half-open")
	}
	cb.recordSuccess()
	if cb.state != stateClosed {
		t.Errorf("state = %d", cb.state)
	}
}
