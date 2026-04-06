package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	serviceSignatureHeader = "X-Service-Signature"
)

var (
	ErrMissingSignature  = errors.New("missing service signature")
	ErrInvalidFormat     = errors.New("invalid signature format")
	ErrExpiredTimestamp   = errors.New("timestamp expired")
	ErrInvalidSignature  = errors.New("invalid signature")
)

// ServiceAuth returns middleware that verifies HMAC-SHA256 request signatures.
// Services sign requests with a shared secret, preventing unauthorized
// internal API access even if network policies are misconfigured.
//
// Signature format in X-Service-Signature header:
//
//	t=<unix_timestamp>,s=<service_name>,v1=<hmac_hex>
//
// HMAC is computed over: timestamp + "." + service_name + "." + method + "." + path
// Timestamp tolerance prevents replay attacks.
func ServiceAuth(secret []byte, tolerance time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sig := r.Header.Get(serviceSignatureHeader)

			serviceName, err := VerifyServiceSignature(r.Method, r.URL.Path, sig, secret, tolerance)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"unauthorized","message":"%s"}`, err.Error()), http.StatusUnauthorized)
				return
			}

			r.Header.Set("X-Service-Name", serviceName)
			next.ServeHTTP(w, r)
		})
	}
}

// SignRequest adds HMAC signature headers to an outgoing request.
func SignRequest(req *http.Request, serviceName string, secret []byte) {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	payload := timestamp + "." + serviceName + "." + req.Method + "." + req.URL.Path

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	signature := hex.EncodeToString(mac.Sum(nil))

	req.Header.Set(serviceSignatureHeader, fmt.Sprintf("t=%s,s=%s,v1=%s", timestamp, serviceName, signature))
}

// VerifyServiceSignature verifies a service signature.
// Returns the service name if valid, or an error.
func VerifyServiceSignature(method, path, signature string, secret []byte, tolerance time.Duration) (string, error) {
	if signature == "" {
		return "", ErrMissingSignature
	}

	parts := parseSignature(signature)
	timestamp, ok := parts["t"]
	if !ok {
		return "", ErrInvalidFormat
	}
	serviceName, ok := parts["s"]
	if !ok {
		return "", ErrInvalidFormat
	}
	sig, ok := parts["v1"]
	if !ok {
		return "", ErrInvalidFormat
	}

	// Validate timestamp
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return "", ErrInvalidFormat
	}

	age := time.Since(time.Unix(ts, 0))
	if age < 0 {
		age = -age
	}
	if age > tolerance {
		return "", ErrExpiredTimestamp
	}

	// Compute expected HMAC
	payload := timestamp + "." + serviceName + "." + method + "." + path
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return "", ErrInvalidSignature
	}

	return serviceName, nil
}

// parseSignature parses "t=123,s=svc,v1=abc" into a map.
func parseSignature(sig string) map[string]string {
	result := make(map[string]string)
	for _, part := range strings.Split(sig, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			result[kv[0]] = kv[1]
		}
	}
	return result
}
