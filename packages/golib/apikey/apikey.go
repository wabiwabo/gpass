// Package apikey provides API key generation, hashing, and
// validation. Keys are shown to the user once in plaintext,
// then stored as SHA-256 hashes. Supports key prefixes for
// identification and expiration.
package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"
)

// Key represents an API key with metadata.
type Key struct {
	ID        string    `json:"id"`
	Prefix    string    `json:"prefix"`     // visible prefix (e.g., "gp_live_")
	Hash      string    `json:"hash"`       // SHA-256 hash of the full key
	Name      string    `json:"name"`       // human-readable name
	Scopes    []string  `json:"scopes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	LastUsed  time.Time `json:"last_used,omitempty"`
	Active    bool      `json:"active"`
}

// IsExpired checks if the key has expired.
func (k Key) IsExpired() bool {
	if k.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(k.ExpiresAt)
}

// IsValid checks if the key is active and not expired.
func (k Key) IsValid() bool {
	return k.Active && !k.IsExpired()
}

// HasScope checks if the key has a specific scope.
func (k Key) HasScope(scope string) bool {
	for _, s := range k.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// GenerateResult contains the plaintext key and its metadata.
type GenerateResult struct {
	Plaintext string `json:"plaintext"` // shown once, never stored
	Key       Key    `json:"key"`
}

// Generate creates a new API key with the given prefix.
// The plaintext is returned once and should be shown to the user.
// Only the hash should be stored.
func Generate(prefix, name string) (GenerateResult, error) {
	if prefix == "" {
		prefix = "gp_"
	}
	if !strings.HasSuffix(prefix, "_") {
		prefix += "_"
	}

	// Generate 32 random bytes (256 bits of entropy)
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return GenerateResult{}, fmt.Errorf("apikey: %w", err)
	}

	secret := hex.EncodeToString(b)
	plaintext := prefix + secret

	// Generate a short ID
	idBytes := make([]byte, 8)
	if _, err := io.ReadFull(rand.Reader, idBytes); err != nil {
		return GenerateResult{}, fmt.Errorf("apikey: %w", err)
	}

	return GenerateResult{
		Plaintext: plaintext,
		Key: Key{
			ID:        hex.EncodeToString(idBytes),
			Prefix:    prefix,
			Hash:      HashKey(plaintext),
			Name:      name,
			CreatedAt: time.Now().UTC(),
			Active:    true,
		},
	}, nil
}

// HashKey computes the SHA-256 hash of an API key.
func HashKey(plaintext string) string {
	h := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(h[:])
}

// Verify checks if a plaintext key matches a stored hash.
// Uses constant-time comparison to prevent timing attacks.
func Verify(plaintext, storedHash string) bool {
	computed := HashKey(plaintext)
	return subtle.ConstantTimeCompare([]byte(computed), []byte(storedHash)) == 1
}

// ExtractPrefix returns the prefix from a plaintext API key.
// e.g., "gp_live_abc123..." → "gp_live_"
func ExtractPrefix(plaintext string) string {
	// Find the last underscore that's part of the prefix
	lastUnderscore := strings.LastIndex(plaintext, "_")
	if lastUnderscore == -1 || lastUnderscore >= len(plaintext)-1 {
		return ""
	}

	// Prefix includes the trailing underscore
	prefix := plaintext[:lastUnderscore+1]

	// Validate prefix has only alphanumeric and underscores
	for _, c := range prefix {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return ""
		}
	}
	return prefix
}

// MaskKey returns a masked version of the key showing only prefix + first 4 chars.
// e.g., "gp_live_abc123def456..." → "gp_live_abc1...".
func MaskKey(plaintext string) string {
	prefix := ExtractPrefix(plaintext)
	if prefix == "" {
		if len(plaintext) > 8 {
			return plaintext[:4] + "..." + plaintext[len(plaintext)-4:]
		}
		return "****"
	}

	secret := plaintext[len(prefix):]
	if len(secret) > 4 {
		return prefix + secret[:4] + "..."
	}
	return prefix + "****"
}
