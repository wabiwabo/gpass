package oss

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSearchByNPWP_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/nib/search" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}

		var req NIBSearchRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.NPWP != "01.234.567.8-901.000" {
			t.Errorf("NPWP = %q, want %q", req.NPWP, "01.234.567.8-901.000")
		}

		resp := NIBSearchResponse{
			Found:  true,
			NIB:    "1234567890123",
			NPWP:   "01.234.567.8-901.000",
			Name:   "PT Test Corp",
			Status: "ACTIVE",
			Businesses: []Business{
				{KBLI: "62011", Description: "IT Consulting", Status: "ACTIVE", RiskLevel: "LOW"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key", 5*time.Second)
	resp, err := client.SearchByNPWP(context.Background(), "01.234.567.8-901.000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Found {
		t.Error("expected Found to be true")
	}
	if resp.NIB != "1234567890123" {
		t.Errorf("NIB = %q, want %q", resp.NIB, "1234567890123")
	}
	if len(resp.Businesses) != 1 {
		t.Fatalf("expected 1 business, got %d", len(resp.Businesses))
	}
}

func TestSearchByNIB_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req NIBSearchRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.NIB != "1234567890123" {
			t.Errorf("NIB = %q, want %q", req.NIB, "1234567890123")
		}

		resp := NIBSearchResponse{
			Found:  true,
			NIB:    "1234567890123",
			Name:   "PT Test Corp",
			Status: "ACTIVE",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "", 5*time.Second)
	resp, err := client.SearchByNIB(context.Background(), "1234567890123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Found {
		t.Error("expected Found to be true")
	}
}

func TestSearchByNPWP_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(NIBSearchResponse{Found: false, Message: "not found"})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "", 5*time.Second)
	resp, err := client.SearchByNPWP(context.Background(), "00.000.000.0-000.000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Found {
		t.Error("expected Found to be false")
	}
}

func TestClient_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "", 5*time.Second)
	_, err := client.SearchByNPWP(context.Background(), "01.234.567.8-901.000")
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestClient_CircuitBreaker(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "", 5*time.Second)

	// Trigger 5 failures to open circuit breaker
	for i := 0; i < 5; i++ {
		client.SearchByNPWP(context.Background(), "test")
	}

	// Next call should fail with circuit breaker open
	_, err := client.SearchByNPWP(context.Background(), "test")
	if err == nil {
		t.Fatal("expected circuit breaker error")
	}
	if callCount != 5 {
		t.Errorf("expected 5 server calls, got %d", callCount)
	}
}
