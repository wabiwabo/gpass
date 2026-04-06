// Package reqsign provides HMAC-based HTTP request signing for
// service-to-service authentication. The sender signs requests with
// a shared secret, and the receiver verifies the signature.
package reqsign

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	// HeaderSignature is the header containing the signature.
	HeaderSignature = "X-Signature"
	// HeaderTimestamp is the header containing the request timestamp.
	HeaderTimestamp = "X-Signature-Timestamp"
	// DefaultMaxAge is the maximum age of a signed request (5 minutes).
	DefaultMaxAge = 5 * time.Minute
)

// Signer signs HTTP requests.
type Signer struct {
	secret []byte
}

// NewSigner creates a request signer with the given secret.
func NewSigner(secret []byte) *Signer {
	return &Signer{secret: secret}
}

// Sign adds signature headers to a request.
func (s *Signer) Sign(req *http.Request) {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	payload := buildPayload(req.Method, req.URL.Path, timestamp)

	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(payload))
	signature := hex.EncodeToString(mac.Sum(nil))

	req.Header.Set(HeaderSignature, signature)
	req.Header.Set(HeaderTimestamp, timestamp)
}

// Verifier verifies signed HTTP requests.
type Verifier struct {
	secret []byte
	maxAge time.Duration
}

// NewVerifier creates a request verifier.
func NewVerifier(secret []byte, maxAge time.Duration) *Verifier {
	if maxAge <= 0 {
		maxAge = DefaultMaxAge
	}
	return &Verifier{secret: secret, maxAge: maxAge}
}

// Verify checks the signature on a request.
// Returns nil if valid, error otherwise.
func (v *Verifier) Verify(req *http.Request) error {
	signature := req.Header.Get(HeaderSignature)
	if signature == "" {
		return fmt.Errorf("reqsign: missing signature header")
	}

	timestamp := req.Header.Get(HeaderTimestamp)
	if timestamp == "" {
		return fmt.Errorf("reqsign: missing timestamp header")
	}

	// Check timestamp freshness.
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("reqsign: invalid timestamp")
	}
	age := time.Since(time.Unix(ts, 0))
	if age > v.maxAge || age < -v.maxAge {
		return fmt.Errorf("reqsign: request too old (%v)", age)
	}

	// Compute expected signature.
	payload := buildPayload(req.Method, req.URL.Path, timestamp)
	mac := hmac.New(sha256.New, v.secret)
	mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))

	// Constant-time comparison.
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return fmt.Errorf("reqsign: invalid signature")
	}

	return nil
}

// Middleware returns HTTP middleware that verifies request signatures.
func (v *Verifier) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := v.Verify(r); err != nil {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintf(w, `{"type":"about:blank","title":"Unauthorized","status":401,"detail":"%s"}`, err.Error())
			return
		}
		next.ServeHTTP(w, r)
	})
}

func buildPayload(method, path, timestamp string) string {
	return strings.Join([]string{method, path, timestamp}, "\n")
}

// RoundTripper wraps http.RoundTripper to automatically sign requests.
type RoundTripper struct {
	Base   http.RoundTripper
	Signer *Signer
}

// RoundTrip implements http.RoundTripper with request signing.
func (rt *RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	base := rt.Base
	if base == nil {
		base = http.DefaultTransport
	}
	rt.Signer.Sign(req)
	return base.RoundTrip(req)
}
