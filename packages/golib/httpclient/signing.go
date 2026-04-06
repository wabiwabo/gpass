package httpclient

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// SignedClient wraps an HTTP client that signs all outgoing requests.
// Signature format: X-Signature: algorithm=hmac-sha256,timestamp=<unix>,signature=<hex>
// Signs over: method + path + timestamp + body_hash
type SignedClient struct {
	client *http.Client
	apiKey string
	secret []byte
}

// NewSignedClient creates a new SignedClient with the given API key, HMAC secret, and timeout.
func NewSignedClient(apiKey string, secret []byte, timeout time.Duration) *SignedClient {
	return &SignedClient{
		client: &http.Client{Timeout: timeout},
		apiKey: apiKey,
		secret: secret,
	}
}

// Do executes a signed request. If body is non-nil, it is JSON-encoded.
func (c *SignedClient) Do(ctx context.Context, method, rawURL string, body interface{}) (*http.Response, error) {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}

	// Parse the URL to get the path for signing
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}

	timestamp := time.Now().Unix()
	sig := computeSignature(c.secret, method, parsed.Path, timestamp, bodyBytes)

	req.Header.Set("X-Signature", fmt.Sprintf("algorithm=hmac-sha256,timestamp=%d,signature=%s", timestamp, sig))

	return c.client.Do(req)
}

// computeSignature computes the HMAC-SHA256 signature over method+path+timestamp+body_hash.
func computeSignature(secret []byte, method, path string, timestamp int64, body []byte) string {
	bodyHash := sha256.Sum256(body)

	message := fmt.Sprintf("%s\n%s\n%d\n%s",
		method,
		path,
		timestamp,
		hex.EncodeToString(bodyHash[:]),
	)

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature verifies an incoming signed request (for simulator testing).
// It checks the X-Signature header, validates the timestamp is within tolerance,
// and verifies the HMAC signature using constant-time comparison.
func VerifySignature(r *http.Request, secret []byte, tolerance time.Duration) error {
	sigHeader := r.Header.Get("X-Signature")
	if sigHeader == "" {
		return fmt.Errorf("missing X-Signature header")
	}

	// Parse header: algorithm=hmac-sha256,timestamp=<unix>,signature=<hex>
	parts := make(map[string]string)
	for _, part := range strings.Split(sigHeader, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			parts[kv[0]] = kv[1]
		}
	}

	algorithm, ok := parts["algorithm"]
	if !ok || algorithm != "hmac-sha256" {
		return fmt.Errorf("unsupported or missing algorithm")
	}

	tsStr, ok := parts["timestamp"]
	if !ok {
		return fmt.Errorf("missing timestamp")
	}
	timestamp, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %w", err)
	}

	sig, ok := parts["signature"]
	if !ok {
		return fmt.Errorf("missing signature")
	}

	// Check timestamp tolerance
	now := time.Now().Unix()
	diff := now - timestamp
	if diff < 0 {
		diff = -diff
	}
	if time.Duration(diff)*time.Second > tolerance {
		return fmt.Errorf("signature expired: timestamp differs by %ds", diff)
	}

	// Read body for verification
	var bodyBytes []byte
	if r.Body != nil {
		bodyBytes, err = io.ReadAll(r.Body)
		if err != nil {
			return fmt.Errorf("read body: %w", err)
		}
		// Restore the body for downstream handlers
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	expected := computeSignature(secret, r.Method, r.URL.Path, timestamp, bodyBytes)

	expectedBytes, err := hex.DecodeString(expected)
	if err != nil {
		return fmt.Errorf("decode expected signature: %w", err)
	}
	actualBytes, err := hex.DecodeString(sig)
	if err != nil {
		return fmt.Errorf("decode actual signature: %w", err)
	}

	if !hmac.Equal(expectedBytes, actualBytes) {
		return fmt.Errorf("signature mismatch")
	}

	return nil
}
