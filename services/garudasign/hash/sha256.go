package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
)

// ComputeHash computes the SHA-256 hash of the data from r and returns it as a hex-encoded string.
func ComputeHash(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyHash computes the SHA-256 hash of data from r and compares it against expectedHex.
func VerifyHash(r io.Reader, expectedHex string) bool {
	got, err := ComputeHash(r)
	if err != nil {
		return false
	}
	return got == expectedHex
}
