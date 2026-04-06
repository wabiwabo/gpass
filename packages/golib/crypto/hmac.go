package crypto

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
)

// HMACSHA256 computes HMAC-SHA256 and returns hex-encoded result.
func HMACSHA256(message, key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write(message)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyHMACSHA256 verifies an HMAC-SHA256 in constant time.
func VerifyHMACSHA256(message, key []byte, expectedHex string) bool {
	expected, err := hex.DecodeString(expectedHex)
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, key)
	mac.Write(message)
	actual := mac.Sum(nil)

	return subtle.ConstantTimeCompare(actual, expected) == 1
}

// GenerateRandomBytes generates n cryptographically random bytes.
func GenerateRandomBytes(n int) ([]byte, error) {
	if n <= 0 {
		return nil, fmt.Errorf("crypto: n must be positive, got %d", n)
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("crypto: generate random bytes: %w", err)
	}
	return b, nil
}

// GenerateRandomHex generates n random bytes and returns hex-encoded string.
func GenerateRandomHex(n int) (string, error) {
	b, err := GenerateRandomBytes(n)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
