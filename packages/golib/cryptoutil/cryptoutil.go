// Package cryptoutil provides cryptographic utility functions for
// enterprise security: constant-time comparison, secure random generation,
// key derivation, and timing-safe operations.
package cryptoutil

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
)

// ConstantTimeEqual performs a constant-time comparison of two byte slices.
// Prevents timing side-channel attacks on secret comparisons.
func ConstantTimeEqual(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}

// ConstantTimeEqualString performs constant-time string comparison.
func ConstantTimeEqualString(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// SecureRandom generates n cryptographically secure random bytes.
func SecureRandom(n int) ([]byte, error) {
	if n <= 0 {
		return nil, fmt.Errorf("cryptoutil: n must be positive")
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return nil, fmt.Errorf("cryptoutil: random: %w", err)
	}
	return buf, nil
}

// SecureRandomHex generates n random bytes encoded as hex string (2n chars).
func SecureRandomHex(n int) (string, error) {
	b, err := SecureRandom(n)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// SecureRandomBase64 generates n random bytes encoded as base64url string.
func SecureRandomBase64(n int) (string, error) {
	b, err := SecureRandom(n)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HMACSHA256 computes HMAC-SHA256 of data with the given key.
func HMACSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

// HMACSHA512 computes HMAC-SHA512 of data with the given key.
func HMACSHA512(key, data []byte) []byte {
	mac := hmac.New(sha512.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

// VerifyHMAC verifies an HMAC-SHA256 in constant time.
func VerifyHMAC(key, data, expectedMAC []byte) bool {
	computed := HMACSHA256(key, data)
	return hmac.Equal(computed, expectedMAC)
}

// DeriveKey derives a key from a master key and context using HMAC.
// This is a simplified key derivation suitable for generating
// purpose-specific keys from a master key.
func DeriveKey(masterKey []byte, context string, length int) ([]byte, error) {
	if len(masterKey) < 16 {
		return nil, fmt.Errorf("cryptoutil: master key must be at least 16 bytes")
	}
	if length <= 0 || length > 64 {
		return nil, fmt.Errorf("cryptoutil: derived key length must be 1-64")
	}

	// HMAC-based key derivation.
	h := hmac.New(sha256.New, masterKey)
	h.Write([]byte(context))
	derived := h.Sum(nil)

	if length > len(derived) {
		// Use SHA-512 for longer keys.
		h512 := hmac.New(sha512.New, masterKey)
		h512.Write([]byte(context))
		derived = h512.Sum(nil)
	}

	return derived[:length], nil
}

// Hash computes a hash of data using the specified algorithm.
func Hash(algorithm string, data []byte) ([]byte, error) {
	var h hash.Hash
	switch algorithm {
	case "sha256":
		h = sha256.New()
	case "sha384":
		h = sha512.New384()
	case "sha512":
		h = sha512.New()
	default:
		return nil, fmt.Errorf("cryptoutil: unsupported algorithm: %s", algorithm)
	}
	h.Write(data)
	return h.Sum(nil), nil
}

// HashHex computes a hash and returns it as a hex string.
func HashHex(algorithm string, data []byte) (string, error) {
	h, err := Hash(algorithm, data)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h), nil
}

// ZeroBytes securely zeroes a byte slice to prevent secrets
// from lingering in memory.
func ZeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// SecureToken generates a URL-safe token of the specified byte length.
// The returned string is base64url-encoded.
func SecureToken(byteLength int) (string, error) {
	return SecureRandomBase64(byteLength)
}
