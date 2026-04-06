package middleware

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// RequestIDFormat defines the format for generated request IDs.
type RequestIDFormat int

const (
	FormatUUID      RequestIDFormat = iota // UUID v4 (default)
	FormatULID                             // ULID (sortable, timestamp-based)
	FormatSnowflake                        // Snowflake-inspired (timestamp + node + seq)
	FormatPrefixed                         // prefix + UUID (e.g., "req_abc123...")
)

// RequestIDConfig configures request ID generation.
type RequestIDConfig struct {
	Format RequestIDFormat
	Prefix string // for FormatPrefixed
	NodeID int    // for FormatSnowflake (0-1023)
}

// RequestIDWithConfig returns middleware that generates request IDs with the given config.
func RequestIDWithConfig(cfg RequestIDConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-Id")
			if id == "" {
				switch cfg.Format {
				case FormatULID:
					id = GenerateULID()
				case FormatSnowflake:
					id = GenerateSnowflake(cfg.NodeID)
				case FormatPrefixed:
					id = GeneratePrefixed(cfg.Prefix)
				default:
					id = generateUUID()
				}
			}

			ctx := context.WithValue(r.Context(), requestIDKey, id)
			w.Header().Set("X-Request-Id", id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Crockford's base32 alphabet for ULID encoding.
const crockfordBase32 = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// GenerateULID generates a ULID (Universally Unique Lexicographically Sortable Identifier).
// Format: 10 chars timestamp (ms) + 16 chars randomness, Crockford's base32.
func GenerateULID() string {
	now := time.Now().UnixMilli()

	var result [26]byte

	// Encode 48-bit timestamp (10 characters, big-endian).
	ts := uint64(now)
	for i := 9; i >= 0; i-- {
		result[i] = crockfordBase32[ts&0x1f]
		ts >>= 5
	}

	// Encode 80 bits of randomness (16 characters).
	randomBytes := make([]byte, 10)
	rand.Read(randomBytes)
	// Encode 10 random bytes into 16 base32 characters.
	// We process 5 bytes at a time (40 bits = 8 base32 chars).
	encodeBase32Block(result[10:18], randomBytes[0:5])
	encodeBase32Block(result[18:26], randomBytes[5:10])

	return string(result[:])
}

// encodeBase32Block encodes 5 bytes into 8 Crockford base32 characters.
func encodeBase32Block(dst []byte, src []byte) {
	val := uint64(0)
	for _, b := range src {
		val = (val << 8) | uint64(b)
	}
	for i := 7; i >= 0; i-- {
		dst[i] = crockfordBase32[val&0x1f]
		val >>= 5
	}
}

// snowflakeSeq is a global atomic counter for snowflake sequence numbers.
var snowflakeSeq atomic.Int64

// snowflakeEpoch is the custom epoch for snowflake IDs (2024-01-01 UTC).
var snowflakeEpoch = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()

// GenerateSnowflake generates a snowflake-style ID (timestamp + node + sequence).
// Uses a custom epoch (2024-01-01) to fit timestamps in 41 bits.
// Layout: 41 bits relative timestamp | 10 bits node | 13 bits sequence
func GenerateSnowflake(nodeID int) string {
	nodeID = nodeID & 0x3ff // mask to 10 bits (0-1023)
	ts := time.Now().UnixMilli() - snowflakeEpoch
	seq := snowflakeSeq.Add(1)

	// Pack into 64-bit: 41 bits timestamp | 10 bits node | 13 bits sequence
	id := ((ts & 0x1ffffffffff) << 23) | (int64(nodeID) << 13) | (seq & 0x1fff)

	return fmt.Sprintf("%d", id)
}

// GeneratePrefixed generates a prefixed UUID (e.g., "req_550e8400-e29b...").
func GeneratePrefixed(prefix string) string {
	uuid := generateUUID()
	if prefix == "" {
		prefix = "req"
	}
	return prefix + "_" + uuid
}

// generateCryptoInt generates a cryptographically random integer in [0, max).
func generateCryptoInt(max int64) int64 {
	n, err := rand.Int(rand.Reader, big.NewInt(max))
	if err != nil {
		// Fallback to reading random bytes.
		b := make([]byte, 8)
		rand.Read(b)
		return int64(binary.BigEndian.Uint64(b)%uint64(max))
	}
	return n.Int64()
}

// isValidUUID checks if a string is a valid UUID v4 format.
func isValidUUID(s string) bool {
	// UUID format: 8-4-4-4-12 hex chars
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// isValidULID checks if a string is a valid ULID format.
func isValidULID(s string) bool {
	if len(s) != 26 {
		return false
	}
	for _, c := range s {
		if !strings.ContainsRune(crockfordBase32, c) {
			return false
		}
	}
	return true
}
