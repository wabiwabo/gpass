// Package cryptohash provides common cryptographic hash utilities.
// Wraps SHA-256, SHA-512, and HMAC operations with hex-encoded
// output for consistent hash formatting across services.
package cryptohash

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
)

// SHA256 returns the hex-encoded SHA-256 hash of data.
func SHA256(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// SHA256String returns the hex-encoded SHA-256 hash of a string.
func SHA256String(s string) string {
	return SHA256([]byte(s))
}

// SHA512Hex returns the hex-encoded SHA-512 hash of data.
func SHA512Hex(data []byte) string {
	h := sha512.Sum512(data)
	return hex.EncodeToString(h[:])
}

// SHA256Bytes returns the raw SHA-256 hash bytes.
func SHA256Bytes(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

// HMAC256 returns hex-encoded HMAC-SHA256.
func HMAC256(key, data []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}

// HMAC256Bytes returns raw HMAC-SHA256 bytes.
func HMAC256Bytes(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

// HMAC512 returns hex-encoded HMAC-SHA512.
func HMAC512(key, data []byte) string {
	mac := hmac.New(sha512.New, key)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyHMAC256 verifies an HMAC-SHA256 signature in constant time.
func VerifyHMAC256(key, data []byte, expectedHex string) bool {
	expected, err := hex.DecodeString(expectedHex)
	if err != nil {
		return false
	}
	actual := HMAC256Bytes(key, data)
	return hmac.Equal(actual, expected)
}
