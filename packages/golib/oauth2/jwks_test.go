package oauth2

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func testJWKS() JWKS {
	return JWKS{
		Keys: []JWK{
			{
				KID:       "key-1",
				KeyType:   "RSA",
				Algorithm: "RS256",
				Use:       "sig",
				N:         "modulus-base64",
				E:         "AQAB",
			},
			{
				KID:       "key-2",
				KeyType:   "EC",
				Algorithm: "ES256",
				Use:       "sig",
				X:         "x-coord",
				Y:         "y-coord",
				Crv:       "P-256",
			},
		},
	}
}

func TestJWKSClient_FetchAndGetKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testJWKS())
	}))
	defer server.Close()

	client := NewJWKSClient(server.URL, 5*time.Minute)

	key, err := client.GetKey(context.Background(), "key-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key.KID != "key-1" {
		t.Errorf("expected kid=key-1, got %s", key.KID)
	}
	if key.KeyType != "RSA" {
		t.Errorf("expected kty=RSA, got %s", key.KeyType)
	}
	if key.Algorithm != "RS256" {
		t.Errorf("expected alg=RS256, got %s", key.Algorithm)
	}

	// Also fetch key-2.
	key2, err := client.GetKey(context.Background(), "key-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key2.KeyType != "EC" {
		t.Errorf("expected kty=EC, got %s", key2.KeyType)
	}
}

func TestJWKSClient_CacheHit(t *testing.T) {
	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testJWKS())
	}))
	defer server.Close()

	client := NewJWKSClient(server.URL, 5*time.Minute)

	// First call fetches from server.
	_, err := client.GetKey(context.Background(), "key-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Second call should use cache.
	_, err = client.GetKey(context.Background(), "key-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if count := callCount.Load(); count != 1 {
		t.Errorf("expected 1 HTTP call (cache hit), got %d", count)
	}
}

func TestJWKSClient_CacheExpiredTriggersRefresh(t *testing.T) {
	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testJWKS())
	}))
	defer server.Close()

	// Use a very short TTL so cache expires quickly.
	client := NewJWKSClient(server.URL, 1*time.Millisecond)

	_, err := client.GetKey(context.Background(), "key-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait for cache to expire.
	time.Sleep(5 * time.Millisecond)

	_, err = client.GetKey(context.Background(), "key-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if count := callCount.Load(); count < 2 {
		t.Errorf("expected at least 2 HTTP calls after cache expiry, got %d", count)
	}
}

func TestJWKSClient_UnknownKID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testJWKS())
	}))
	defer server.Close()

	client := NewJWKSClient(server.URL, 5*time.Minute)

	_, err := client.GetKey(context.Background(), "nonexistent-kid")
	if err == nil {
		t.Fatal("expected error for unknown kid")
	}
}

func TestJWKSClient_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewJWKSClient(server.URL, 5*time.Minute)

	_, err := client.GetKey(context.Background(), "key-1")
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}
