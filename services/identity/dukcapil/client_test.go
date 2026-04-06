package dukcapil

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestVerifyNIK_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/nik/verify" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(NIKVerifyResponse{
			Valid: true,
			Alive: true,
			Name:  "Budi Santoso",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", 5*time.Second)
	resp, err := client.VerifyNIK(context.Background(), "3201234567890001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Valid {
		t.Error("expected Valid=true")
	}
	if !resp.Alive {
		t.Error("expected Alive=true")
	}
	if resp.Name != "Budi Santoso" {
		t.Errorf("Name = %q, want %q", resp.Name, "Budi Santoso")
	}
}

func TestVerifyNIK_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", 100*time.Millisecond)
	_, err := client.VerifyNIK(context.Background(), "3201234567890001")
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestCircuitBreaker_TripsAfter5Failures(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", 5*time.Second)

	// Make 5 failing calls to trip the circuit breaker
	for i := 0; i < 5; i++ {
		_, err := client.VerifyNIK(context.Background(), "3201234567890001")
		if err == nil {
			t.Fatalf("call %d: expected error", i+1)
		}
	}

	// 6th call should be rejected by circuit breaker without hitting server
	prevCount := callCount
	_, err := client.VerifyNIK(context.Background(), "3201234567890001")
	if err == nil {
		t.Fatal("expected circuit breaker error")
	}
	if callCount != prevCount {
		t.Error("circuit breaker should have prevented the request")
	}
}
