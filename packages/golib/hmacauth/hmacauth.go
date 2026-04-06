// Package hmacauth provides HMAC-based service-to-service
// authentication middleware. Services sign requests with a shared
// secret, and the middleware verifies signatures using constant-time
// comparison to prevent timing attacks.
package hmacauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Config controls HMAC authentication behavior.
type Config struct {
	// Secret is the shared HMAC key.
	Secret []byte
	// HeaderName is the header containing the signature (default "X-Signature").
	HeaderName string
	// TimestampHeader is the header with the request timestamp (default "X-Timestamp").
	TimestampHeader string
	// MaxSkew is the maximum allowed clock skew (default 5 minutes).
	MaxSkew time.Duration
	// SignedHeaders are additional headers to include in the signature.
	SignedHeaders []string
}

// DefaultConfig returns production defaults.
func DefaultConfig(secret []byte) Config {
	return Config{
		Secret:          secret,
		HeaderName:      "X-Signature",
		TimestampHeader: "X-Timestamp",
		MaxSkew:         5 * time.Minute,
	}
}

// Sign generates an HMAC-SHA256 signature for a request.
func Sign(cfg Config, method, path, timestamp string, headers http.Header) string {
	mac := hmac.New(sha256.New, cfg.Secret)

	// Canonical string: method\npath\ntimestamp\nheader1:value1\n...
	mac.Write([]byte(method))
	mac.Write([]byte("\n"))
	mac.Write([]byte(path))
	mac.Write([]byte("\n"))
	mac.Write([]byte(timestamp))

	if len(cfg.SignedHeaders) > 0 {
		sorted := make([]string, len(cfg.SignedHeaders))
		copy(sorted, cfg.SignedHeaders)
		sort.Strings(sorted)

		for _, h := range sorted {
			mac.Write([]byte("\n"))
			mac.Write([]byte(strings.ToLower(h)))
			mac.Write([]byte(":"))
			mac.Write([]byte(headers.Get(h)))
		}
	}

	return hex.EncodeToString(mac.Sum(nil))
}

// Verify checks an HMAC-SHA256 signature using constant-time comparison.
func Verify(cfg Config, method, path, timestamp, signature string, headers http.Header) bool {
	expected := Sign(cfg, method, path, timestamp, headers)
	return subtle.ConstantTimeCompare([]byte(expected), []byte(signature)) == 1
}

// CheckTimestamp verifies the timestamp is within MaxSkew of now.
func CheckTimestamp(timestamp string, maxSkew time.Duration) error {
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return fmt.Errorf("invalid timestamp format: %w", err)
	}

	skew := time.Since(t)
	if skew < 0 {
		skew = -skew
	}

	if skew > maxSkew {
		return fmt.Errorf("timestamp skew %v exceeds maximum %v", skew, maxSkew)
	}

	return nil
}

// Middleware returns HTTP middleware that verifies HMAC signatures.
func Middleware(cfg Config) func(http.Handler) http.Handler {
	if cfg.HeaderName == "" {
		cfg.HeaderName = "X-Signature"
	}
	if cfg.TimestampHeader == "" {
		cfg.TimestampHeader = "X-Timestamp"
	}
	if cfg.MaxSkew <= 0 {
		cfg.MaxSkew = 5 * time.Minute
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			signature := r.Header.Get(cfg.HeaderName)
			if signature == "" {
				http.Error(w, `{"error":"missing signature"}`, http.StatusUnauthorized)
				return
			}

			timestamp := r.Header.Get(cfg.TimestampHeader)
			if timestamp == "" {
				http.Error(w, `{"error":"missing timestamp"}`, http.StatusUnauthorized)
				return
			}

			if err := CheckTimestamp(timestamp, cfg.MaxSkew); err != nil {
				http.Error(w, `{"error":"invalid timestamp"}`, http.StatusUnauthorized)
				return
			}

			if !Verify(cfg, r.Method, r.URL.Path, timestamp, signature, r.Header) {
				http.Error(w, `{"error":"invalid signature"}`, http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SignRequest signs an outgoing HTTP request in place.
func SignRequest(cfg Config, req *http.Request) {
	if cfg.TimestampHeader == "" {
		cfg.TimestampHeader = "X-Timestamp"
	}
	if cfg.HeaderName == "" {
		cfg.HeaderName = "X-Signature"
	}

	timestamp := time.Now().UTC().Format(time.RFC3339)
	req.Header.Set(cfg.TimestampHeader, timestamp)

	sig := Sign(cfg, req.Method, req.URL.Path, timestamp, req.Header)
	req.Header.Set(cfg.HeaderName, sig)
}
