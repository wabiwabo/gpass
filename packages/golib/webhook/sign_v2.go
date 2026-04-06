package webhook

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// V2Signer signs webhook payloads with Ed25519 (stronger than HMAC-SHA256).
type V2Signer struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
}

// NewV2Signer generates a new Ed25519 signing key pair.
func NewV2Signer() (*V2Signer, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("webhook: generate ed25519 key: %w", err)
	}
	return &V2Signer{
		privateKey: priv,
		publicKey:  pub,
	}, nil
}

// NewV2SignerFromKey creates a signer from an existing private key.
func NewV2SignerFromKey(privateKey ed25519.PrivateKey) *V2Signer {
	return &V2Signer{
		privateKey: privateKey,
		publicKey:  privateKey.Public().(ed25519.PublicKey),
	}
}

// Sign signs a webhook payload.
// Format: v2=<hex-encoded-ed25519-signature>
func (s *V2Signer) Sign(payload []byte, timestamp int64) string {
	message := buildSigningMessage(payload, timestamp)
	sig := ed25519.Sign(s.privateKey, message)
	return "v2=" + hex.EncodeToString(sig)
}

// Verify verifies a webhook signature.
func (s *V2Signer) Verify(payload []byte, signature string, timestamp int64, tolerance time.Duration) bool {
	return verifySignature(payload, signature, timestamp, s.publicKey, tolerance)
}

// PublicKeyHex returns the hex-encoded public key for distribution to webhook consumers.
func (s *V2Signer) PublicKeyHex() string {
	return hex.EncodeToString(s.publicKey)
}

// VerifyWithPublicKey verifies using only the public key (for consumers).
func VerifyWithPublicKey(payload []byte, signature string, timestamp int64, publicKeyHex string, tolerance time.Duration) bool {
	pubBytes, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		return false
	}
	if len(pubBytes) != ed25519.PublicKeySize {
		return false
	}
	pubKey := ed25519.PublicKey(pubBytes)
	return verifySignature(payload, signature, timestamp, pubKey, tolerance)
}

// buildSigningMessage creates the message to sign: "timestamp.payload"
func buildSigningMessage(payload []byte, timestamp int64) []byte {
	ts := strconv.FormatInt(timestamp, 10)
	msg := make([]byte, 0, len(ts)+1+len(payload))
	msg = append(msg, ts...)
	msg = append(msg, '.')
	msg = append(msg, payload...)
	return msg
}

// verifySignature performs the actual signature verification.
func verifySignature(payload []byte, signature string, timestamp int64, pubKey ed25519.PublicKey, tolerance time.Duration) bool {
	// Check timestamp tolerance.
	if tolerance > 0 {
		ts := time.Unix(timestamp, 0)
		if time.Since(ts) > tolerance {
			return false
		}
	}

	// Parse signature format "v2=<hex>".
	if !strings.HasPrefix(signature, "v2=") {
		return false
	}
	sigHex := strings.TrimPrefix(signature, "v2=")
	sigBytes, err := hex.DecodeString(sigHex)
	if err != nil {
		return false
	}

	message := buildSigningMessage(payload, timestamp)
	return ed25519.Verify(pubKey, message, sigBytes)
}
