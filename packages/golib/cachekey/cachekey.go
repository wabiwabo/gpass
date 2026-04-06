// Package cachekey provides structured cache key generation for
// consistent key naming across services. Prevents key collisions
// and enables efficient cache invalidation by prefix.
package cachekey

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Builder constructs cache keys with consistent structure.
type Builder struct {
	prefix    string
	separator string
}

// New creates a cache key builder with a service prefix.
func New(prefix string) *Builder {
	return &Builder{
		prefix:    prefix,
		separator: ":",
	}
}

// Key builds a cache key from parts.
// e.g., New("identity").Key("user", userID) → "identity:user:abc123"
func (b *Builder) Key(parts ...string) string {
	all := make([]string, 0, len(parts)+1)
	all = append(all, b.prefix)
	all = append(all, parts...)
	return strings.Join(all, b.separator)
}

// Hash builds a hashed cache key for long/complex values.
func (b *Builder) Hash(parts ...string) string {
	raw := b.Key(parts...)
	h := sha256.Sum256([]byte(raw))
	return b.prefix + b.separator + hex.EncodeToString(h[:8])
}

// Pattern returns a glob pattern for prefix-based invalidation.
// e.g., New("identity").Pattern("user") → "identity:user:*"
func (b *Builder) Pattern(parts ...string) string {
	return b.Key(parts...) + b.separator + "*"
}

// Common key builders for GarudaPass services.

// UserKey builds a key for user-related data.
func UserKey(service, userID string) string {
	return New(service).Key("user", userID)
}

// SessionKey builds a key for session data.
func SessionKey(sessionID string) string {
	return New("session").Key(sessionID)
}

// ConsentKey builds a key for consent data.
func ConsentKey(userID, scope string) string {
	return New("consent").Key(userID, scope)
}

// EntityKey builds a key for corporate entity data.
func EntityKey(entityID string) string {
	return New("entity").Key(entityID)
}

// RateLimitKey builds a key for rate limiting.
func RateLimitKey(endpoint, clientID string) string {
	return New("ratelimit").Key(endpoint, clientID)
}

// OTPKey builds a key for OTP storage.
func OTPKey(userID, channel string) string {
	return New("otp").Key(userID, channel)
}

// CertKey builds a key for certificate data.
func CertKey(userID string) string {
	return New("cert").Key(userID)
}
