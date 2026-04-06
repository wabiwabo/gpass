package token

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

const (
	// base62Alphabet is the character set for base62 encoding (alphanumeric, no ambiguous chars).
	base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

// Generate creates a cryptographically secure random token of the given byte length,
// returned as a hex-encoded string (2x length in chars).
func Generate(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("token: generate random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// GenerateBase64URL creates a URL-safe base64-encoded token of the given byte length.
// No padding characters.
func GenerateBase64URL(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("token: generate random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// GenerateBase62 creates a base62-encoded token of exactly the given character length.
// Base62 uses 0-9, A-Z, a-z — safe for URLs, headers, and filenames.
func GenerateBase62(charLen int) (string, error) {
	alphabetLen := big.NewInt(int64(len(base62Alphabet)))
	var b strings.Builder
	b.Grow(charLen)

	for i := 0; i < charLen; i++ {
		n, err := rand.Int(rand.Reader, alphabetLen)
		if err != nil {
			return "", fmt.Errorf("token: generate base62: %w", err)
		}
		b.WriteByte(base62Alphabet[n.Int64()])
	}

	return b.String(), nil
}

// GenerateUUID creates a version 4 UUID (random).
// Format: xxxxxxxx-xxxx-4xxx-[89ab]xxx-xxxxxxxxxxxx
func GenerateUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("token: generate UUID: %w", err)
	}

	// Set version 4.
	b[6] = (b[6] & 0x0f) | 0x40
	// Set variant 10xx.
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// GenerateAPIKey creates a GarudaPass API key with prefix and checksum.
// Format: gp_{env}_{base62_token}
// Example: gp_live_4k8JxF9mK2pNvR7qS1wT
func GenerateAPIKey(env string, tokenLen int) (string, error) {
	if env != "test" && env != "live" {
		return "", fmt.Errorf("token: env must be 'test' or 'live', got %q", env)
	}
	if tokenLen < 16 {
		tokenLen = 32
	}

	tok, err := GenerateBase62(tokenLen)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("gp_%s_%s", env, tok), nil
}

// GenerateOTP creates a numeric OTP of the given digit length.
func GenerateOTP(digits int) (string, error) {
	if digits < 4 || digits > 10 {
		return "", fmt.Errorf("token: OTP digits must be 4-10, got %d", digits)
	}

	max := big.NewInt(1)
	for i := 0; i < digits; i++ {
		max.Mul(max, big.NewInt(10))
	}

	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("token: generate OTP: %w", err)
	}

	format := fmt.Sprintf("%%0%dd", digits)
	return fmt.Sprintf(format, n.Int64()), nil
}

// Bytes generates raw random bytes.
func Bytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("token: generate bytes: %w", err)
	}
	return b, nil
}
