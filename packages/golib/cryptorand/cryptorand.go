// Package cryptorand provides cryptographically secure random
// generators for tokens, IDs, passwords, and OTP codes using
// crypto/rand. All functions are safe for concurrent use.
package cryptorand

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"strings"
)

// Bytes returns n cryptographically random bytes.
func Bytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return nil, fmt.Errorf("cryptorand: %w", err)
	}
	return b, nil
}

// Hex returns a hex-encoded random string of n random bytes (2n chars).
func Hex(n int) (string, error) {
	b, err := Bytes(n)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Base64 returns a URL-safe base64-encoded random string of n random bytes.
func Base64(n int) (string, error) {
	b, err := Bytes(n)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// Base64Raw returns a URL-safe base64-encoded string without padding.
func Base64Raw(n int) (string, error) {
	b, err := Bytes(n)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Token generates a 32-byte (256-bit) hex token suitable for API keys,
// session tokens, and CSRF tokens.
func Token() (string, error) {
	return Hex(32)
}

// OTP generates a numeric one-time password of the specified length.
// Length must be between 4 and 10.
func OTP(length int) (string, error) {
	if length < 4 || length > 10 {
		return "", fmt.Errorf("cryptorand: OTP length must be 4-10, got %d", length)
	}

	// Calculate max value (10^length)
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(length)), nil)

	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("cryptorand: %w", err)
	}

	// Left-pad with zeros
	format := fmt.Sprintf("%%0%dd", length)
	return fmt.Sprintf(format, n), nil
}

// ID generates a URL-safe random ID of approximately the given byte entropy.
func ID(entropyBytes int) (string, error) {
	if entropyBytes < 1 {
		entropyBytes = 16
	}
	return Base64Raw(entropyBytes)
}

// Choose picks a cryptographically random element from the slice.
func Choose[T any](items []T) (T, error) {
	var zero T
	if len(items) == 0 {
		return zero, fmt.Errorf("cryptorand: empty slice")
	}

	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(items))))
	if err != nil {
		return zero, fmt.Errorf("cryptorand: %w", err)
	}

	return items[n.Int64()], nil
}

// Shuffle randomly permutes the slice in place using Fisher-Yates.
func Shuffle[T any](items []T) error {
	for i := len(items) - 1; i > 0; i-- {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return fmt.Errorf("cryptorand: %w", err)
		}
		j := int(n.Int64())
		items[i], items[j] = items[j], items[i]
	}
	return nil
}

// Password generates a random password of the given length containing
// uppercase, lowercase, digits, and special characters.
func Password(length int) (string, error) {
	if length < 8 {
		return "", fmt.Errorf("cryptorand: password length must be >= 8, got %d", length)
	}

	const (
		upper   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		lower   = "abcdefghijklmnopqrstuvwxyz"
		digits  = "0123456789"
		special = "!@#$%^&*()-_=+[]{}|;:,.<>?"
		all     = upper + lower + digits + special
	)

	// Ensure at least one of each category
	var password strings.Builder
	password.Grow(length)

	charSets := []string{upper, lower, digits, special}
	for _, cs := range charSets {
		c, err := chooseFromString(cs)
		if err != nil {
			return "", err
		}
		password.WriteByte(c)
	}

	// Fill remaining with random from all
	for i := 4; i < length; i++ {
		c, err := chooseFromString(all)
		if err != nil {
			return "", err
		}
		password.WriteByte(c)
	}

	// Shuffle the result
	result := []byte(password.String())
	for i := len(result) - 1; i > 0; i-- {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return "", fmt.Errorf("cryptorand: %w", err)
		}
		j := int(n.Int64())
		result[i], result[j] = result[j], result[i]
	}

	return string(result), nil
}

func chooseFromString(s string) (byte, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(s))))
	if err != nil {
		return 0, fmt.Errorf("cryptorand: %w", err)
	}
	return s[n.Int64()], nil
}
