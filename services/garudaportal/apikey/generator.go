package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
)

const (
	base62Alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	randomBytes    = 48
	prefixLen      = 16
)

// GenerateKey generates an API key for the given environment.
// Returns the plaintext key, SHA-256 hash (hex), and display prefix.
func GenerateKey(environment string) (plaintext, hash, prefix string, err error) {
	keyPrefix, err := envPrefix(environment)
	if err != nil {
		return "", "", "", err
	}

	// Generate 48 random bytes
	buf := make([]byte, randomBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", "", fmt.Errorf("crypto/rand: %w", err)
	}

	// Encode as base62
	encoded := encodeBase62(buf)

	// Build full key
	plaintext = keyPrefix + encoded

	// SHA-256 hash
	h := sha256.Sum256([]byte(plaintext))
	hash = hex.EncodeToString(h[:])

	// Prefix: first 16 chars of full key
	prefix = plaintext[:prefixLen]

	return plaintext, hash, prefix, nil
}

// HashKey computes the SHA-256 hash of an API key.
func HashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

func envPrefix(env string) (string, error) {
	switch env {
	case "sandbox":
		return "gp_test_", nil
	case "production":
		return "gp_live_", nil
	default:
		return "", fmt.Errorf("invalid environment: %q (must be sandbox or production)", env)
	}
}

// encodeBase62 converts arbitrary bytes to a base62 string.
func encodeBase62(data []byte) string {
	num := new(big.Int).SetBytes(data)
	base := big.NewInt(62)
	zero := big.NewInt(0)
	mod := new(big.Int)

	if num.Cmp(zero) == 0 {
		return string(base62Alphabet[0])
	}

	var result []byte
	for num.Cmp(zero) > 0 {
		num.DivMod(num, base, mod)
		result = append(result, base62Alphabet[mod.Int64()])
	}

	// Reverse
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return string(result)
}
