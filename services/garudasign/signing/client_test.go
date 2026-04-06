package signing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIssueCertificate_Success(t *testing.T) {
	expected := CertificateIssueResponse{
		SerialNumber:      "123456",
		CertificatePEM:    "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----",
		IssuerDN:          "CN=Test CA",
		SubjectDN:         "CN=Test User",
		ValidFrom:         "2025-01-01T00:00:00Z",
		ValidTo:           "2026-01-01T00:00:00Z",
		FingerprintSHA256: "abcdef",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/certificates/issue" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, 5*time.Second)
	resp, err := client.IssueCertificate(context.Background(), CertificateIssueRequest{
		SubjectCN:    "Test User",
		SubjectUID:   "user-123",
		ValidityDays: 365,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.SerialNumber != expected.SerialNumber {
		t.Errorf("expected serial %s, got %s", expected.SerialNumber, resp.SerialNumber)
	}
}

func TestSignDocument_Success(t *testing.T) {
	expected := SignResponse{
		SignedDocumentBase64: "c2lnbmVk",
		SignatureTimestamp:   "2025-01-01T00:00:00Z",
		PAdESLevel:          "PAdES-B-LTA",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sign/pades" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, 5*time.Second)
	resp, err := client.SignDocument(context.Background(), SignRequest{
		DocumentBase64: "dGVzdA==",
		CertificatePEM: "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----",
		SignatureLevel: "PAdES-B-LTA",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.PAdESLevel != expected.PAdESLevel {
		t.Errorf("expected level %s, got %s", expected.PAdESLevel, resp.PAdESLevel)
	}
}

func TestIssueCertificate_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, 5*time.Second)
	_, err := client.IssueCertificate(context.Background(), CertificateIssueRequest{
		SubjectCN: "Test",
	})
	if err == nil {
		t.Fatal("expected error for server error")
	}
}

func TestCircuitBreaker_TripsAfter5Failures(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, 5*time.Second)

	// Trip the circuit breaker with 5 failures
	for i := 0; i < 5; i++ {
		client.IssueCertificate(context.Background(), CertificateIssueRequest{SubjectCN: "Test"})
	}

	// The 6th call should fail with circuit breaker error
	_, err := client.IssueCertificate(context.Background(), CertificateIssueRequest{SubjectCN: "Test"})
	if err == nil {
		t.Fatal("expected circuit breaker error")
	}
	if err.Error() != "circuit breaker open: service unavailable" {
		t.Errorf("unexpected error message: %v", err)
	}
}
