package dukcapil

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestVerifyBiometric_HappyPath pins the previously-0% method via a real
// httptest server that returns the expected JSON shape.
func TestVerifyBiometric_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req BiometricRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.NIK != "3171012345670001" || req.SelfieB64 != "data:image/jpeg;base64,abc" {
			t.Errorf("unexpected payload: %+v", req)
		}
		if r.Header.Get("X-API-Key") != "test-key" {
			t.Errorf("API key not propagated")
		}
		json.NewEncoder(w).Encode(BiometricResponse{Match: true, Confidence: 0.95})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key", 5*time.Second)
	resp, err := c.VerifyBiometric(context.Background(), "3171012345670001", "data:image/jpeg;base64,abc")
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Match || resp.Confidence != 0.95 {
		t.Errorf("resp = %+v", resp)
	}
}

// TestVerifyDemographic_HappyPath pins the previously-0% method.
func TestVerifyDemographic_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req DemographicRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.NIK == "" {
			t.Error("NIK missing")
		}
		json.NewEncoder(w).Encode(DemographicResponse{Match: true, Score: 0.99})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k", 5*time.Second)
	resp, err := c.VerifyDemographic(context.Background(), &DemographicRequest{
		NIK: "3171012345670001", Name: "Alice", BirthDate: "1990-01-01", BirthPlace: "Jakarta",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Match || resp.Score != 0.99 {
		t.Errorf("resp = %+v", resp)
	}
}

// TestCircuitBreaker_OpensAfterThreshold pins the recordFailure →
// stateOpen transition AND the allow() rejection while open.
func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`{"error":"boom"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "", 2*time.Second)
	// 5 consecutive failures (default threshold) should open the breaker.
	for i := 0; i < 5; i++ {
		_, err := c.VerifyNIK(context.Background(), "3171012345670001")
		if err == nil {
			t.Errorf("call %d: expected error", i)
		}
	}
	// Next call must be rejected by the open breaker, NOT reach the server.
	_, err := c.VerifyNIK(context.Background(), "3171012345670001")
	if err == nil || !strings.Contains(err.Error(), "circuit breaker open") {
		t.Errorf("breaker not open: err = %v", err)
	}
}

// TestCircuitBreaker_HalfOpenAllowsOneCall pins the openUntil expiry →
// stateHalfOpen transition. We use a tiny cooldown so the test is fast.
func TestCircuitBreaker_HalfOpenAllowsOneCall(t *testing.T) {
	cb := newCircuitBreaker(2, 20*time.Millisecond)
	cb.recordFailure()
	cb.recordFailure()
	if cb.allow() {
		t.Error("breaker should be open immediately after threshold")
	}
	time.Sleep(40 * time.Millisecond)
	if !cb.allow() {
		t.Error("breaker should transition to half-open after cooldown")
	}
	// Half-open allows further calls until success/failure resolves.
	if !cb.allow() {
		t.Error("half-open should keep allowing until resolved")
	}
	cb.recordSuccess()
	if cb.state != stateClosed {
		t.Errorf("state after success = %d, want stateClosed", cb.state)
	}
}
