package ahu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestGetOfficers_HappyPath pins the doGet success branch.
func TestGetOfficers_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s", r.Method)
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("Accept missing")
		}
		if r.Header.Get("X-API-Key") != "test-key" {
			t.Errorf("API key missing")
		}
		json.NewEncoder(w).Encode(OfficersResponse{Officers: []Officer{
			{Name: "Alice", Position: "Director"},
		}})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key", 5*time.Second)
	out, err := c.GetOfficers(context.Background(), "AHU-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Name != "Alice" {
		t.Errorf("got %+v", out)
	}
}

// TestGetShareholders_HappyPath pins the doGet success branch.
func TestGetShareholders_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(ShareholdersResponse{Shareholders: []Shareholder{
			{Name: "Bob", Percentage: 51.0},
		}})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "", 5*time.Second)
	out, err := c.GetShareholders(context.Background(), "AHU-2")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Percentage != 51.0 {
		t.Errorf("got %+v", out)
	}
}

// TestDoGet_StatusError pins the non-200 branch in doRequest via doGet.
func TestDoGet_StatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(503)
		w.Write([]byte("temporary unavailable"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k", 5*time.Second)
	_, err := c.GetOfficers(context.Background(), "AHU-X")
	if err == nil || !strings.Contains(err.Error(), "status 503") {
		t.Errorf("err = %v", err)
	}
}

// TestCircuitBreaker_OpensAfterThreshold pins the recordFailure → stateOpen
// transition AND the rejection-while-open contract.
func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "", 2*time.Second)
	for i := 0; i < 5; i++ {
		_, err := c.SearchCompany(context.Background(), "AHU-Y")
		if err == nil {
			t.Errorf("call %d: expected error", i)
		}
	}
	// 6th call must be rejected by the open breaker.
	_, err := c.SearchCompany(context.Background(), "AHU-Y")
	if err == nil || !strings.Contains(err.Error(), "circuit breaker open") {
		t.Errorf("breaker not open: %v", err)
	}
}

// TestCircuitBreaker_HalfOpenTransition pins the openUntil expiry path.
func TestCircuitBreaker_HalfOpenTransition(t *testing.T) {
	cb := newCircuitBreaker(2, 20*time.Millisecond)
	cb.recordFailure()
	cb.recordFailure()
	if cb.allow() {
		t.Error("breaker should be open after threshold")
	}
	time.Sleep(40 * time.Millisecond)
	if !cb.allow() {
		t.Error("breaker should transition to half-open")
	}
	cb.recordSuccess()
	if cb.state != stateClosed {
		t.Errorf("state after success = %d, want closed", cb.state)
	}
}

// TestDoGet_DialFailure pins the httpClient.Do error branch.
func TestDoGet_DialFailure(t *testing.T) {
	c := NewClient("http://127.0.0.1:1", "", 200*time.Millisecond)
	_, err := c.GetOfficers(context.Background(), "AHU-Z")
	if err == nil || !strings.Contains(err.Error(), "AHU request failed") {
		t.Errorf("err = %v", err)
	}
}
