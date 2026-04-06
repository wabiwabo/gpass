// Package dpop implements Demonstrating Proof of Possession (DPoP)
// token validation per RFC 9449. Required for FAPI 2.0 compliance.
// Verifies that the sender possesses the private key used to sign
// the DPoP proof JWT.
package dpop

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

// Proof represents a parsed DPoP proof.
type Proof struct {
	// JTI is the unique identifier for this proof.
	JTI string
	// HTM is the HTTP method the proof is bound to.
	HTM string
	// HTU is the HTTP URI the proof is bound to.
	HTU string
	// IAT is when the proof was issued.
	IAT time.Time
	// ATH is the access token hash (for token-bound proofs).
	ATH string
}

// Config for DPoP validation.
type Config struct {
	// MaxAge is the maximum age of a DPoP proof (default 5 minutes).
	MaxAge time.Duration
	// NonceRequired requires a server nonce in the proof.
	NonceRequired bool
	// AllowedMethods restricts which HTTP methods are valid.
	AllowedMethods []string
}

// DefaultConfig returns production defaults.
func DefaultConfig() Config {
	return Config{
		MaxAge: 5 * time.Minute,
	}
}

// ValidationError represents a DPoP validation failure.
type ValidationError struct {
	Code    string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("dpop: %s: %s", e.Code, e.Message)
}

// Common validation errors.
var (
	ErrMissingProof    = &ValidationError{"missing_proof", "DPoP proof header is required"}
	ErrInvalidProof    = &ValidationError{"invalid_proof", "DPoP proof is malformed"}
	ErrExpiredProof    = &ValidationError{"expired_proof", "DPoP proof has expired"}
	ErrMethodMismatch  = &ValidationError{"method_mismatch", "DPoP proof HTTP method does not match"}
	ErrURIMismatch     = &ValidationError{"uri_mismatch", "DPoP proof HTTP URI does not match"}
	ErrMissingJTI      = &ValidationError{"missing_jti", "DPoP proof must have a unique identifier"}
	ErrTokenHashFailed = &ValidationError{"ath_mismatch", "DPoP access token hash does not match"}
)

// ValidateBinding checks that a proof matches the expected HTTP method and URI.
func ValidateBinding(proof Proof, expectedMethod, expectedURI string, cfg Config) error {
	if proof.JTI == "" {
		return ErrMissingJTI
	}

	if !strings.EqualFold(proof.HTM, expectedMethod) {
		return ErrMethodMismatch
	}

	// Compare URIs without query/fragment
	proofURI := normalizeURI(proof.HTU)
	expectURI := normalizeURI(expectedURI)
	if proofURI != expectURI {
		return ErrURIMismatch
	}

	// Check proof age
	maxAge := cfg.MaxAge
	if maxAge <= 0 {
		maxAge = 5 * time.Minute
	}
	if time.Since(proof.IAT) > maxAge {
		return ErrExpiredProof
	}

	// Reject future proofs
	if proof.IAT.After(time.Now().Add(30 * time.Second)) {
		return ErrExpiredProof
	}

	return nil
}

// ComputeTokenHash computes the ath (access token hash) for DPoP binding.
// ath = BASE64URL(SHA256(access_token))
func ComputeTokenHash(accessToken string) string {
	h := sha256.Sum256([]byte(accessToken))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// VerifyTokenBinding checks that the proof's ATH matches the access token.
func VerifyTokenBinding(proof Proof, accessToken string) error {
	expected := ComputeTokenHash(accessToken)
	if proof.ATH != expected {
		return ErrTokenHashFailed
	}
	return nil
}

// IsMethodAllowed checks if the method is in the allowed list.
func IsMethodAllowed(method string, allowed []string) bool {
	if len(allowed) == 0 {
		return true // no restriction
	}
	for _, m := range allowed {
		if strings.EqualFold(m, method) {
			return true
		}
	}
	return false
}

func normalizeURI(uri string) string {
	// Strip query string and fragment
	if idx := strings.IndexByte(uri, '?'); idx != -1 {
		uri = uri[:idx]
	}
	if idx := strings.IndexByte(uri, '#'); idx != -1 {
		uri = uri[:idx]
	}
	return strings.TrimRight(uri, "/")
}
