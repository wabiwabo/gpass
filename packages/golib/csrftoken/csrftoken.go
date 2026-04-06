// Package csrftoken provides CSRF token generation and validation
// using the double-submit cookie pattern. Tokens are HMAC-signed
// with constant-time comparison to prevent timing attacks.
package csrftoken

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"
)

// Config controls CSRF token behavior.
type Config struct {
	Secret []byte
	TTL    time.Duration
}

// DefaultTTL is the default token validity period.
const DefaultTTL = 1 * time.Hour

// Generate creates a new CSRF token.
// Format: base64(random):base64(timestamp):hmac
func Generate(cfg Config) (string, error) {
	nonce := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("csrftoken: %w", err)
	}

	nonceStr := base64.RawURLEncoding.EncodeToString(nonce)
	tsStr := fmt.Sprintf("%d", time.Now().Unix())
	payload := nonceStr + ":" + tsStr
	sig := sign(cfg.Secret, payload)

	return payload + ":" + sig, nil
}

// Validate checks if a CSRF token is valid and not expired.
func Validate(cfg Config, token string) error {
	parts := strings.SplitN(token, ":", 3)
	if len(parts) != 3 {
		return fmt.Errorf("csrftoken: invalid format")
	}

	payload := parts[0] + ":" + parts[1]
	sig := parts[2]

	expectedSig := sign(cfg.Secret, payload)
	if subtle.ConstantTimeCompare([]byte(expectedSig), []byte(sig)) != 1 {
		return fmt.Errorf("csrftoken: invalid signature")
	}

	var ts int64
	if _, err := fmt.Sscanf(parts[1], "%d", &ts); err != nil {
		return fmt.Errorf("csrftoken: invalid timestamp")
	}

	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = DefaultTTL
	}

	age := time.Since(time.Unix(ts, 0))
	if age > ttl {
		return fmt.Errorf("csrftoken: token expired")
	}
	if age < -30*time.Second {
		return fmt.Errorf("csrftoken: token from future")
	}

	return nil
}

func sign(secret []byte, payload string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
