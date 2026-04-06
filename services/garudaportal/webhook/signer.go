package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Sign computes an HMAC-SHA256 signature for a webhook payload.
// Returns a signature in the format "t={timestamp},v1={hmac}".
func Sign(payload []byte, secret string, timestamp int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	msg := fmt.Sprintf("%d.%s", timestamp, payload)
	mac.Write([]byte(msg))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("t=%d,v1=%s", timestamp, sig)
}

// Verify verifies a webhook signature against the payload and secret.
// Returns true if the signature is valid and within the time tolerance.
func Verify(payload []byte, secret string, signature string, tolerance time.Duration) bool {
	// Parse signature
	ts, sig, err := parseSignature(signature)
	if err != nil {
		return false
	}

	// Check timestamp tolerance
	signedAt := time.Unix(ts, 0)
	if time.Since(signedAt) > tolerance {
		return false
	}

	// Compute expected signature
	mac := hmac.New(sha256.New, []byte(secret))
	msg := fmt.Sprintf("%d.%s", ts, payload)
	mac.Write([]byte(msg))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(sig), []byte(expectedSig))
}

func parseSignature(signature string) (timestamp int64, sig string, err error) {
	parts := strings.Split(signature, ",")
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("invalid signature format")
	}

	tsPart := parts[0]
	sigPart := parts[1]

	if !strings.HasPrefix(tsPart, "t=") {
		return 0, "", fmt.Errorf("missing timestamp")
	}
	if !strings.HasPrefix(sigPart, "v1=") {
		return 0, "", fmt.Errorf("missing v1 signature")
	}

	ts, err := strconv.ParseInt(tsPart[2:], 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("invalid timestamp: %w", err)
	}

	return ts, sigPart[3:], nil
}
