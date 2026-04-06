// Package hashutil provides common hashing utilities built on
// crypto/sha256 and crypto/sha512. Convenience functions for
// hashing strings, combining hashes, and creating deterministic IDs.
package hashutil

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"strings"
)

// SHA256 returns the SHA-256 hex digest of a string.
func SHA256(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// SHA256Bytes returns the SHA-256 hex digest of bytes.
func SHA256Bytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// SHA512 returns the SHA-512 hex digest of a string.
func SHA512(s string) string {
	h := sha512.Sum512([]byte(s))
	return hex.EncodeToString(h[:])
}

// SHA256Short returns the first n hex chars of a SHA-256 hash.
func SHA256Short(s string, n int) string {
	full := SHA256(s)
	if n > len(full) {
		return full
	}
	return full[:n]
}

// Combine creates a deterministic hash from multiple values.
func Combine(parts ...string) string {
	return SHA256(strings.Join(parts, "\x00"))
}

// DeterministicID creates a short deterministic ID from input.
// Returns first 16 hex chars of SHA-256 (64 bits).
func DeterministicID(parts ...string) string {
	return SHA256Short(strings.Join(parts, ":"), 16)
}

// Equal compares two hex hash strings in constant time.
func Equal(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var result byte
	for i := 0; i < len(a); i++ {
		result |= a[i] ^ b[i]
	}
	return result == 0
}
