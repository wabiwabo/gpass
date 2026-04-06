// Package base62 provides base62 encoding/decoding for generating
// URL-safe, compact identifiers. Uses [0-9A-Za-z] character set.
// Commonly used for short URLs, invite codes, and public-facing IDs.
package base62

import (
	"fmt"
	"math/big"
	"strings"
)

const charset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

var base = big.NewInt(62)

// Encode encodes bytes to a base62 string.
func Encode(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	n := new(big.Int).SetBytes(data)
	if n.Sign() == 0 {
		return string(charset[0])
	}

	var result []byte
	mod := new(big.Int)
	zero := big.NewInt(0)

	for n.Cmp(zero) > 0 {
		n.DivMod(n, base, mod)
		result = append(result, charset[mod.Int64()])
	}

	// Preserve leading zeros
	for _, b := range data {
		if b != 0 {
			break
		}
		result = append(result, charset[0])
	}

	// Reverse
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return string(result)
}

// Decode decodes a base62 string to bytes.
func Decode(s string) ([]byte, error) {
	if s == "" {
		return nil, nil
	}

	n := new(big.Int)
	for _, c := range s {
		idx := strings.IndexRune(charset, c)
		if idx < 0 {
			return nil, fmt.Errorf("base62: invalid character %q", c)
		}
		n.Mul(n, base)
		n.Add(n, big.NewInt(int64(idx)))
	}

	return n.Bytes(), nil
}

// EncodeInt encodes an integer to base62.
func EncodeInt(n uint64) string {
	if n == 0 {
		return string(charset[0])
	}

	var result []byte
	for n > 0 {
		result = append(result, charset[n%62])
		n /= 62
	}

	// Reverse
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return string(result)
}

// DecodeInt decodes a base62 string to an integer.
func DecodeInt(s string) (uint64, error) {
	if s == "" {
		return 0, fmt.Errorf("base62: empty string")
	}

	var n uint64
	for _, c := range s {
		idx := strings.IndexRune(charset, c)
		if idx < 0 {
			return 0, fmt.Errorf("base62: invalid character %q", c)
		}
		n = n*62 + uint64(idx)
	}

	return n, nil
}

// IsValid checks if a string contains only base62 characters.
func IsValid(s string) bool {
	for _, c := range s {
		if strings.IndexRune(charset, c) < 0 {
			return false
		}
	}
	return true
}
