package signing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestSignDocument_HappyPath pins the previously-75% SignDocument success.
func TestSignDocument_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/sign/pades") {
			t.Errorf("path = %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(SignResponse{
			SignedDocumentBase64: "ZmFrZS1zaWduZWQ=",
			SignatureTimestamp:   "2026-04-07T00:00:00Z",
			PAdESLevel:           "PAdES-B-LTA",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, 5*time.Second)
	resp, err := c.SignDocument(context.Background(), SignRequest{
		DocumentBase64: "ZmFrZQ==",
		CertificatePEM: "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.PAdESLevel != "PAdES-B-LTA" {
		t.Errorf("got %+v", resp)
	}
}

// TestDoPost_4xxClientError pins the 4xx-with-error-body branch.
func TestDoPost_4xxClientError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"error":"bad_input","message":"missing field"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, 5*time.Second)
	_, err := c.IssueCertificate(context.Background(), CertificateIssueRequest{})
	if err == nil || !strings.Contains(err.Error(), "client error") || !strings.Contains(err.Error(), "missing field") {
		t.Errorf("err = %v", err)
	}
}

// TestDoPost_5xxRecordsFailure pins the 5xx → recordFailure branch and
// the eventual circuit-breaker open transition after 5 failures.
func TestDoPost_5xxOpensCircuit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(503)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, 5*time.Second)
	for i := 0; i < 5; i++ {
		_, err := c.IssueCertificate(context.Background(), CertificateIssueRequest{})
		if err == nil {
			t.Errorf("call %d: expected error", i)
		}
	}
	// 6th call must be rejected by checkCircuit.
	_, err := c.IssueCertificate(context.Background(), CertificateIssueRequest{})
	if err == nil || !strings.Contains(err.Error(), "circuit breaker open") {
		t.Errorf("breaker not open: %v", err)
	}
}

// TestCheckCircuit_CooldownExpiryResets pins the lastFailedAt+cooldown
// expiry branch — after the cooldown elapses, calls must be allowed
// again and the failure counter must reset.
func TestCheckCircuit_CooldownExpiryResets(t *testing.T) {
	c := NewClient("http://x", 1*time.Second)
	c.cooldown = 20 * time.Millisecond
	c.maxFailures = 2

	c.recordFailure()
	c.recordFailure()
	if err := c.checkCircuit(); err == nil {
		t.Error("breaker should be open")
	}

	time.Sleep(40 * time.Millisecond)
	if err := c.checkCircuit(); err != nil {
		t.Errorf("after cooldown: %v", err)
	}
	if c.failureCount != 0 || c.circuitOpen {
		t.Errorf("state not reset: count=%d open=%v", c.failureCount, c.circuitOpen)
	}
}

// TestDoPost_DialFailureRecordsFailure pins the httpClient.Do error path.
func TestDoPost_DialFailureRecordsFailure(t *testing.T) {
	c := NewClient("http://127.0.0.1:1", 200*time.Millisecond)
	_, err := c.IssueCertificate(context.Background(), CertificateIssueRequest{})
	if err == nil || !strings.Contains(err.Error(), "request failed") {
		t.Errorf("err = %v", err)
	}
	if c.failureCount != 1 {
		t.Errorf("failureCount = %d, want 1", c.failureCount)
	}
}
