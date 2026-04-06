package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

var testSecret = []byte("test-shared-secret-for-services-32bytes!")

func makeSignature(t *testing.T, method, path, serviceName string, secret []byte, ts time.Time) string {
	t.Helper()
	timestamp := strconv.FormatInt(ts.Unix(), 10)
	payload := timestamp + "." + serviceName + "." + method + "." + path
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("t=%s,s=%s,v1=%s", timestamp, serviceName, sig)
}

func TestServiceAuth_ValidSignature(t *testing.T) {
	handler := ServiceAuth(testSecret, 5*time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.Header.Set(serviceSignatureHeader, makeSignature(t, http.MethodGet, "/api/v1/users", "identity-svc", testSecret, time.Now()))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestServiceAuth_MissingSignature(t *testing.T) {
	handler := ServiceAuth(testSecret, 5*time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestServiceAuth_TamperedSignature(t *testing.T) {
	handler := ServiceAuth(testSecret, 5*time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	sig := makeSignature(t, http.MethodGet, "/api/v1/users", "identity-svc", testSecret, time.Now())
	// Tamper with the signature
	sig = sig[:len(sig)-4] + "dead"

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.Header.Set(serviceSignatureHeader, sig)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestServiceAuth_ExpiredTimestamp(t *testing.T) {
	handler := ServiceAuth(testSecret, 5*time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	// Use a timestamp 10 minutes in the past
	sig := makeSignature(t, http.MethodGet, "/api/v1/users", "identity-svc", testSecret, time.Now().Add(-10*time.Minute))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.Header.Set(serviceSignatureHeader, sig)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestSignRequest_RoundTrip(t *testing.T) {
	handler := ServiceAuth(testSecret, 5*time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Service-Name"); got != "consent-svc" {
			t.Errorf("got service name %q, want %q", got, "consent-svc")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/consent/grant", nil)
	SignRequest(req, "consent-svc", testSecret)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestDifferentMethodsProduceDifferentSignatures(t *testing.T) {
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	SignRequest(req1, "svc", testSecret)

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/users", nil)
	SignRequest(req2, "svc", testSecret)

	sig1 := req1.Header.Get(serviceSignatureHeader)
	sig2 := req2.Header.Get(serviceSignatureHeader)

	if sig1 == sig2 {
		t.Error("GET and POST should produce different signatures")
	}
}

func TestServiceNameExtracted(t *testing.T) {
	var gotServiceName string
	handler := ServiceAuth(testSecret, 5*time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotServiceName = r.Header.Get("X-Service-Name")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/logs", nil)
	SignRequest(req, "audit-service", testSecret)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if gotServiceName != "audit-service" {
		t.Errorf("got service name %q, want %q", gotServiceName, "audit-service")
	}
}
