// Package base32x provides extended base32 encoding utilities.
// Supports standard (RFC 4648) and hex encodings with padding
// control for URL-safe token generation.
package base32x

import (
	"encoding/base32"
	"strings"
)

// Encode encodes bytes to standard base32 with padding.
func Encode(data []byte) string {
	return base32.StdEncoding.EncodeToString(data)
}

// Decode decodes a standard base32 string.
func Decode(s string) ([]byte, error) {
	return base32.StdEncoding.DecodeString(s)
}

// EncodeNoPad encodes bytes to base32 without padding.
func EncodeNoPad(data []byte) string {
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(data)
}

// DecodeNoPad decodes a base32 string without padding.
func DecodeNoPad(s string) ([]byte, error) {
	return base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(s)
}

// EncodeHex encodes bytes to base32hex (RFC 4648 extended hex).
func EncodeHex(data []byte) string {
	return base32.HexEncoding.EncodeToString(data)
}

// DecodeHex decodes a base32hex string.
func DecodeHex(s string) ([]byte, error) {
	return base32.HexEncoding.DecodeString(s)
}

// EncodeLower encodes to lowercase base32 without padding.
func EncodeLower(data []byte) string {
	return strings.ToLower(EncodeNoPad(data))
}

// DecodeLower decodes a lowercase base32 string.
func DecodeLower(s string) ([]byte, error) {
	return DecodeNoPad(strings.ToUpper(s))
}

// IsValid checks if a string is valid base32 (with or without padding).
func IsValid(s string) bool {
	if s == "" {
		return false
	}
	_, err := Decode(s)
	if err == nil {
		return true
	}
	_, err = DecodeNoPad(s)
	return err == nil
}
