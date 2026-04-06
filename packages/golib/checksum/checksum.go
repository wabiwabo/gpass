// Package checksum provides data integrity verification using
// cryptographic hashes. Supports SHA-256 and SHA-512 checksums
// for files, byte slices, and streaming data.
package checksum

import (
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"hash"
	"io"
)

// Algorithm represents a hash algorithm.
type Algorithm string

const (
	SHA256 Algorithm = "sha256"
	SHA512 Algorithm = "sha512"
)

// Sum computes a hex-encoded checksum of data.
func Sum(data []byte, alg Algorithm) string {
	h := newHash(alg)
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// SumReader computes a checksum from a reader.
func SumReader(r io.Reader, alg Algorithm) (string, error) {
	h := newHash(alg)
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Verify checks if data matches an expected checksum.
// Uses constant-time comparison.
func Verify(data []byte, expected string, alg Algorithm) bool {
	actual := Sum(data, alg)
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}

// VerifyReader checks if reader content matches an expected checksum.
func VerifyReader(r io.Reader, expected string, alg Algorithm) (bool, error) {
	actual, err := SumReader(r, alg)
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1, nil
}

// SHA256Sum is a convenience function for SHA-256 checksum.
func SHA256Sum(data []byte) string {
	return Sum(data, SHA256)
}

// SHA512Sum is a convenience function for SHA-512 checksum.
func SHA512Sum(data []byte) string {
	return Sum(data, SHA512)
}

func newHash(alg Algorithm) hash.Hash {
	switch alg {
	case SHA512:
		return sha512.New()
	default:
		return sha256.New()
	}
}
