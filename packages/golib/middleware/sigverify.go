package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// SignVerifyConfig configures the request signature verification middleware.
type SignVerifyConfig struct {
	// Secret is the shared HMAC secret for signature verification.
	Secret []byte
	// Tolerance is the maximum age of a valid signature. Default: 5 minutes.
	Tolerance time.Duration
	// SkipPaths are paths that bypass signature verification.
	SkipPaths []string
}

// SignVerify returns middleware that verifies HMAC-SHA256 request signatures.
// Compatible with httpclient.SignedClient signature format:
// X-Signature: algorithm=hmac-sha256,timestamp=<unix>,signature=<hex>
// Signs over: method\npath\ntimestamp\nsha256(body)
func SignVerify(cfg SignVerifyConfig) func(http.Handler) http.Handler {
	if cfg.Tolerance == 0 {
		cfg.Tolerance = 5 * time.Minute
	}

	skipSet := make(map[string]bool, len(cfg.SkipPaths))
	for _, p := range cfg.SkipPaths {
		skipSet[p] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if skipSet[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			if err := verifyRequestSignature(r, cfg.Secret, cfg.Tolerance); err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"signature_invalid","message":"%s"}`, err.Error()), http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func verifyRequestSignature(r *http.Request, secret []byte, tolerance time.Duration) error {
	sigHeader := r.Header.Get("X-Signature")
	if sigHeader == "" {
		return fmt.Errorf("missing X-Signature header")
	}

	// Parse: algorithm=hmac-sha256,timestamp=<unix>,signature=<hex>
	parts := make(map[string]string)
	for _, part := range strings.Split(sigHeader, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			parts[kv[0]] = kv[1]
		}
	}

	algorithm := parts["algorithm"]
	if algorithm != "hmac-sha256" {
		return fmt.Errorf("unsupported algorithm: %s", algorithm)
	}

	tsStr, ok := parts["timestamp"]
	if !ok {
		return fmt.Errorf("missing timestamp")
	}
	timestamp, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp")
	}

	sig, ok := parts["signature"]
	if !ok {
		return fmt.Errorf("missing signature")
	}

	// Verify timestamp tolerance.
	now := time.Now().Unix()
	diff := now - timestamp
	if diff < 0 {
		diff = -diff
	}
	if time.Duration(diff)*time.Second > tolerance {
		return fmt.Errorf("signature expired")
	}

	// Read body for verification.
	var bodyBytes []byte
	if r.Body != nil {
		bodyBytes, err = io.ReadAll(r.Body)
		if err != nil {
			return fmt.Errorf("read body: %w", err)
		}
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	// Compute expected signature.
	bodyHash := sha256.Sum256(bodyBytes)
	message := fmt.Sprintf("%s\n%s\n%d\n%s",
		r.Method,
		r.URL.Path,
		timestamp,
		hex.EncodeToString(bodyHash[:]),
	)

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(message))
	expected := mac.Sum(nil)

	actual, err := hex.DecodeString(sig)
	if err != nil {
		return fmt.Errorf("invalid signature encoding")
	}

	if !hmac.Equal(expected, actual) {
		return fmt.Errorf("signature mismatch")
	}

	return nil
}
