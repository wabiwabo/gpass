package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSignedClient_DoAddsSignatureHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sig := r.Header.Get("X-Signature")
		if sig == "" {
			t.Error("expected X-Signature header to be set")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewSignedClient("test-key", []byte("test-secret"), 5*time.Second)
	resp, err := client.Do(context.Background(), http.MethodGet, server.URL+"/api/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestSignedClient_SignatureContainsRequiredParts(t *testing.T) {
	var capturedSig string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSig = r.Header.Get("X-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewSignedClient("test-key", []byte("test-secret"), 5*time.Second)
	resp, err := client.Do(context.Background(), http.MethodPost, server.URL+"/api/test", map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if !strings.Contains(capturedSig, "algorithm=hmac-sha256") {
		t.Errorf("signature missing algorithm: %s", capturedSig)
	}
	if !strings.Contains(capturedSig, "timestamp=") {
		t.Errorf("signature missing timestamp: %s", capturedSig)
	}
	if !strings.Contains(capturedSig, "signature=") {
		t.Errorf("signature missing signature value: %s", capturedSig)
	}
}

func TestVerifySignature_ValidRequest(t *testing.T) {
	secret := []byte("shared-secret")
	body := map[string]string{"nik": "1234567890123456"}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := VerifySignature(r, secret, 5*time.Minute); err != nil {
			t.Errorf("expected valid signature, got error: %v", err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewSignedClient("key", secret, 5*time.Second)
	resp, err := client.Do(context.Background(), http.MethodPost, server.URL+"/api/dukcapil/verify", body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestVerifySignature_TamperedBody(t *testing.T) {
	secret := []byte("shared-secret")

	// Create a valid signature for the original body
	originalBytes, _ := json.Marshal(map[string]string{"nik": "1234567890123456"})
	timestamp := time.Now().Unix()
	sig := computeSignature(secret, http.MethodPost, "/api/test", timestamp, originalBytes)

	// Use a different body (tampered) with the original signature
	tamperedBytes := []byte(`{"nik":"9999999999999999"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/test", bytes.NewReader(tamperedBytes))
	req.Header.Set("X-Signature", fmt.Sprintf("algorithm=hmac-sha256,timestamp=%d,signature=%s", timestamp, sig))

	err := VerifySignature(req, secret, 5*time.Minute)
	if err == nil {
		t.Error("expected verification to fail for tampered body")
	}
	if err != nil && !strings.Contains(err.Error(), "signature mismatch") {
		t.Errorf("expected signature mismatch error, got: %v", err)
	}
}

func TestVerifySignature_ExpiredTimestamp(t *testing.T) {
	secret := []byte("shared-secret")

	// Create a signature with an old timestamp (10 minutes ago)
	oldTimestamp := time.Now().Add(-10 * time.Minute).Unix()
	body := []byte(`{"test":"data"}`)
	sig := computeSignature(secret, http.MethodPost, "/api/test", oldTimestamp, body)

	req := httptest.NewRequest(http.MethodPost, "/api/test", bytes.NewReader(body))
	req.Header.Set("X-Signature", fmt.Sprintf("algorithm=hmac-sha256,timestamp=%d,signature=%s", oldTimestamp, sig))

	err := VerifySignature(req, secret, 5*time.Minute)
	if err == nil {
		t.Error("expected verification to fail for expired timestamp")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("expected expired error, got: %v", err)
	}
}

func TestSignedClient_DifferentMethodsProduceDifferentSignatures(t *testing.T) {
	signatures := make(map[string]string)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		signatures[r.Method] = r.Header.Get("X-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewSignedClient("key", []byte("secret"), 5*time.Second)

	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete} {
		resp, err := client.Do(context.Background(), method, server.URL+"/api/test", nil)
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", method, err)
		}
		resp.Body.Close()
	}

	// Extract just the signature part for comparison
	getSigValue := func(header string) string {
		for _, part := range strings.Split(header, ",") {
			if strings.HasPrefix(part, "signature=") {
				return strings.TrimPrefix(part, "signature=")
			}
		}
		return ""
	}

	seen := make(map[string]string)
	for method, header := range signatures {
		sigVal := getSigValue(header)
		if prev, exists := seen[sigVal]; exists {
			t.Errorf("methods %s and %s produced the same signature", prev, method)
		}
		seen[sigVal] = method
	}
}

func TestVerifySignature_MissingHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	err := VerifySignature(req, []byte("secret"), 5*time.Minute)
	if err == nil {
		t.Error("expected error for missing X-Signature header")
	}
}

func TestSignedClient_SetsAPIKeyHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "my-api-key" {
			t.Errorf("expected X-API-Key to be my-api-key, got %s", r.Header.Get("X-API-Key"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewSignedClient("my-api-key", []byte("secret"), 5*time.Second)
	resp, err := client.Do(context.Background(), http.MethodGet, server.URL+"/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
}

func TestVerifySignature_BodyRestoredAfterVerification(t *testing.T) {
	secret := []byte("test-secret")
	body := map[string]string{"key": "value"}
	bodyBytes, _ := json.Marshal(body)

	timestamp := time.Now().Unix()
	sig := computeSignature(secret, http.MethodPost, "/api/test", timestamp, bodyBytes)

	req := httptest.NewRequest(http.MethodPost, "/api/test", bytes.NewReader(bodyBytes))
	req.Header.Set("X-Signature", fmt.Sprintf("algorithm=hmac-sha256,timestamp=%d,signature=%s", timestamp, sig))

	err := VerifySignature(req, secret, 5*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Body should still be readable after verification
	restoredBody, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("failed to read restored body: %v", err)
	}
	if !bytes.Equal(restoredBody, bodyBytes) {
		t.Errorf("body not restored after verification")
	}
}
