// Package consttime provides constant-time comparison functions
// for security-sensitive operations. Prevents timing side-channel
// attacks when comparing secrets, tokens, and hashes.
package consttime

import (
	"crypto/subtle"
)

// Equal compares two strings in constant time.
func Equal(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// EqualBytes compares two byte slices in constant time.
func EqualBytes(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}

// Select returns a if selector is 1, b if selector is 0.
// Selector must be 0 or 1.
func Select(selector int, a, b string) string {
	if subtle.ConstantTimeSelect(selector, 1, 0) == 1 {
		return a
	}
	return b
}

// IsZero checks if all bytes are zero in constant time.
func IsZero(data []byte) bool {
	var v byte
	for _, b := range data {
		v |= b
	}
	return subtle.ConstantTimeByteEq(v, 0) == 1
}

// LengthEqual checks if two strings have equal length.
// This leaks length information but not content.
func LengthEqual(a, b string) bool {
	return subtle.ConstantTimeEq(int32(len(a)), int32(len(b))) == 1
}
