// Package hmacverify provides general-purpose HMAC verification
// utilities. Supports multiple hash algorithms, canonical string
// construction, and constant-time comparison. Used for webhook
// verification, API signing, and inter-service authentication.
package hmacverify

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"hash"
	"strings"
)

// Algorithm represents a supported HMAC algorithm.
type Algorithm string

const (
	SHA256 Algorithm = "sha256"
	SHA512 Algorithm = "sha512"
)

// Sign computes an HMAC signature for the given parts joined by separator.
func Sign(secret []byte, alg Algorithm, separator string, parts ...string) string {
	mac := hmac.New(hashFunc(alg), secret)
	mac.Write([]byte(strings.Join(parts, separator)))
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify checks a signature using constant-time comparison.
func Verify(secret []byte, alg Algorithm, separator, signature string, parts ...string) bool {
	expected := Sign(secret, alg, separator, parts...)
	return subtle.ConstantTimeCompare([]byte(expected), []byte(signature)) == 1
}

// SignBytes computes an HMAC signature for raw bytes.
func SignBytes(secret []byte, alg Algorithm, data []byte) string {
	mac := hmac.New(hashFunc(alg), secret)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyBytes checks a signature for raw bytes.
func VerifyBytes(secret []byte, alg Algorithm, data []byte, signature string) bool {
	expected := SignBytes(secret, alg, data)
	return subtle.ConstantTimeCompare([]byte(expected), []byte(signature)) == 1
}

func hashFunc(alg Algorithm) func() hash.Hash {
	switch alg {
	case SHA512:
		return sha512.New
	default:
		return sha256.New
	}
}
