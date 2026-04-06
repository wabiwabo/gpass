// Package pkce implements Proof Key for Code Exchange (RFC 7636)
// for OAuth2 PKCE S256 challenge/verifier generation and validation.
// Required for FAPI 2.0 compliance.
package pkce

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"regexp"
)

// Method represents the PKCE challenge method.
type Method string

const (
	// MethodS256 is the SHA-256 challenge method (required for FAPI 2.0).
	MethodS256 Method = "S256"
	// MethodPlain is the plain challenge method (not recommended).
	MethodPlain Method = "plain"
)

// verifierPattern validates code verifier: [A-Za-z0-9-._~]{43,128}
var verifierPattern = regexp.MustCompile(`^[A-Za-z0-9\-._~]{43,128}$`)

// Pair holds a PKCE verifier and its challenge.
type Pair struct {
	Verifier  string `json:"code_verifier"`
	Challenge string `json:"code_challenge"`
	Method    Method `json:"code_challenge_method"`
}

// Generate creates a new PKCE verifier/challenge pair using S256.
func Generate() (Pair, error) {
	verifier, err := generateVerifier(43)
	if err != nil {
		return Pair{}, err
	}

	challenge := ChallengeS256(verifier)
	return Pair{
		Verifier:  verifier,
		Challenge: challenge,
		Method:    MethodS256,
	}, nil
}

// GenerateWithLength creates a PKCE pair with a specific verifier length.
// Length must be between 43 and 128 per RFC 7636.
func GenerateWithLength(length int) (Pair, error) {
	if length < 43 || length > 128 {
		return Pair{}, fmt.Errorf("pkce: verifier length must be 43-128, got %d", length)
	}

	verifier, err := generateVerifier(length)
	if err != nil {
		return Pair{}, err
	}

	challenge := ChallengeS256(verifier)
	return Pair{
		Verifier:  verifier,
		Challenge: challenge,
		Method:    MethodS256,
	}, nil
}

// ChallengeS256 computes the S256 challenge from a verifier.
// challenge = BASE64URL(SHA256(verifier))
func ChallengeS256(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// ChallengePlain returns the plain challenge (verifier itself).
func ChallengePlain(verifier string) string {
	return verifier
}

// Verify validates a code verifier against a challenge.
func Verify(verifier, challenge string, method Method) bool {
	switch method {
	case MethodS256:
		return ChallengeS256(verifier) == challenge
	case MethodPlain:
		return verifier == challenge
	default:
		return false
	}
}

// ValidVerifier checks if a string is a valid PKCE code verifier.
func ValidVerifier(v string) bool {
	return verifierPattern.MatchString(v)
}

// ValidMethod checks if a challenge method is supported.
func ValidMethod(m Method) bool {
	return m == MethodS256 || m == MethodPlain
}

func generateVerifier(length int) (string, error) {
	// Generate enough random bytes and base64url encode
	// base64url produces ~4/3 ratio, so we need ~3/4 * length bytes
	numBytes := (length*3)/4 + 1
	b := make([]byte, numBytes)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", fmt.Errorf("pkce: %w", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(b)
	if len(encoded) < length {
		return "", fmt.Errorf("pkce: generated verifier too short")
	}
	return encoded[:length], nil
}
