// Package randstring provides secure random string generation
// with configurable character sets. Built on crypto/rand for
// cryptographic security.
package randstring

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// CharSet defines available character sets.
type CharSet string

const (
	Alphanumeric CharSet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	Alpha        CharSet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	Numeric      CharSet = "0123456789"
	Hex          CharSet = "0123456789abcdef"
	LowerAlpha   CharSet = "abcdefghijklmnopqrstuvwxyz"
	UpperAlpha   CharSet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	URLSafe      CharSet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
)

// Generate returns a random string of the given length from the charset.
func Generate(length int, charset CharSet) (string, error) {
	if length <= 0 {
		return "", nil
	}
	if len(charset) == 0 {
		return "", fmt.Errorf("randstring: empty charset")
	}

	result := make([]byte, length)
	max := big.NewInt(int64(len(charset)))

	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("randstring: %w", err)
		}
		result[i] = charset[n.Int64()]
	}

	return string(result), nil
}

// Alphanumeric returns a random alphanumeric string.
func AlphanumericString(length int) (string, error) {
	return Generate(length, Alphanumeric)
}

// HexString returns a random hex string.
func HexString(length int) (string, error) {
	return Generate(length, Hex)
}

// NumericString returns a random numeric string.
func NumericString(length int) (string, error) {
	return Generate(length, Numeric)
}

// URLSafeString returns a random URL-safe string.
func URLSafeString(length int) (string, error) {
	return Generate(length, URLSafe)
}
