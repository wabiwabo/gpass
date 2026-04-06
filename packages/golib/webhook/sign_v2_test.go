package webhook

import (
	"crypto/ed25519"
	"testing"
	"time"
)

func TestSign_VerifyRoundTrip(t *testing.T) {
	signer, err := NewV2Signer()
	if err != nil {
		t.Fatalf("NewV2Signer: %v", err)
	}

	payload := []byte(`{"event":"user.created","data":{"id":"123"}}`)
	ts := time.Now().Unix()

	sig := signer.Sign(payload, ts)
	if !signer.Verify(payload, sig, ts, 5*time.Minute) {
		t.Error("valid signature should verify")
	}
}

func TestVerify_TamperedPayloadFails(t *testing.T) {
	signer, err := NewV2Signer()
	if err != nil {
		t.Fatalf("NewV2Signer: %v", err)
	}

	payload := []byte(`{"event":"user.created"}`)
	ts := time.Now().Unix()
	sig := signer.Sign(payload, ts)

	tampered := []byte(`{"event":"user.deleted"}`)
	if signer.Verify(tampered, sig, ts, 5*time.Minute) {
		t.Error("tampered payload should not verify")
	}
}

func TestVerify_ExpiredTimestampFails(t *testing.T) {
	signer, err := NewV2Signer()
	if err != nil {
		t.Fatalf("NewV2Signer: %v", err)
	}

	payload := []byte(`{"event":"test"}`)
	oldTs := time.Now().Add(-10 * time.Minute).Unix()
	sig := signer.Sign(payload, oldTs)

	// With 5-minute tolerance, a 10-minute-old timestamp should fail.
	if signer.Verify(payload, sig, oldTs, 5*time.Minute) {
		t.Error("expired timestamp should not verify")
	}
}

func TestVerifyWithPublicKey_Works(t *testing.T) {
	signer, err := NewV2Signer()
	if err != nil {
		t.Fatalf("NewV2Signer: %v", err)
	}

	payload := []byte(`{"event":"payment.completed"}`)
	ts := time.Now().Unix()
	sig := signer.Sign(payload, ts)
	pubHex := signer.PublicKeyHex()

	if !VerifyWithPublicKey(payload, sig, ts, pubHex, 5*time.Minute) {
		t.Error("VerifyWithPublicKey should succeed with correct public key")
	}

	// Tampered payload should fail.
	if VerifyWithPublicKey([]byte("tampered"), sig, ts, pubHex, 5*time.Minute) {
		t.Error("VerifyWithPublicKey should fail with tampered payload")
	}
}

func TestSign_DifferentPayloadsProduceDifferentSignatures(t *testing.T) {
	signer, err := NewV2Signer()
	if err != nil {
		t.Fatalf("NewV2Signer: %v", err)
	}

	ts := time.Now().Unix()
	sig1 := signer.Sign([]byte("payload-1"), ts)
	sig2 := signer.Sign([]byte("payload-2"), ts)

	if sig1 == sig2 {
		t.Error("different payloads should produce different signatures")
	}
}

func TestNewV2Signer_GeneratesUniqueKeyPairs(t *testing.T) {
	s1, err := NewV2Signer()
	if err != nil {
		t.Fatalf("NewV2Signer: %v", err)
	}
	s2, err := NewV2Signer()
	if err != nil {
		t.Fatalf("NewV2Signer: %v", err)
	}

	if s1.PublicKeyHex() == s2.PublicKeyHex() {
		t.Error("different signers should have different key pairs")
	}
}

func TestPublicKeyHex_CorrectLength(t *testing.T) {
	signer, err := NewV2Signer()
	if err != nil {
		t.Fatalf("NewV2Signer: %v", err)
	}

	pubHex := signer.PublicKeyHex()
	// Ed25519 public key is 32 bytes = 64 hex characters.
	if len(pubHex) != 64 {
		t.Errorf("expected public key hex length 64, got %d", len(pubHex))
	}
}

func TestNewV2SignerFromKey(t *testing.T) {
	// Generate a key pair manually.
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	signer := NewV2SignerFromKey(priv)
	payload := []byte("test-payload")
	ts := time.Now().Unix()

	sig := signer.Sign(payload, ts)
	if !signer.Verify(payload, sig, ts, 5*time.Minute) {
		t.Error("signer from key should produce valid signatures")
	}
}

func TestVerify_ZeroTolerance_NoTimestampCheck(t *testing.T) {
	signer, err := NewV2Signer()
	if err != nil {
		t.Fatalf("NewV2Signer: %v", err)
	}

	payload := []byte("test")
	oldTs := time.Now().Add(-24 * time.Hour).Unix()
	sig := signer.Sign(payload, oldTs)

	// With zero tolerance, timestamp check is skipped.
	if !signer.Verify(payload, sig, oldTs, 0) {
		t.Error("zero tolerance should skip timestamp check")
	}
}

func TestVerify_InvalidSignatureFormat(t *testing.T) {
	signer, err := NewV2Signer()
	if err != nil {
		t.Fatalf("NewV2Signer: %v", err)
	}

	payload := []byte("test")
	ts := time.Now().Unix()

	if signer.Verify(payload, "invalid", ts, 5*time.Minute) {
		t.Error("invalid signature format should fail")
	}
	if signer.Verify(payload, "v2=notvalidhex!!!", ts, 5*time.Minute) {
		t.Error("invalid hex should fail")
	}
}

func TestVerifyWithPublicKey_InvalidPublicKey(t *testing.T) {
	if VerifyWithPublicKey([]byte("test"), "v2=aabb", time.Now().Unix(), "invalid-hex", 5*time.Minute) {
		t.Error("invalid public key hex should fail")
	}
	if VerifyWithPublicKey([]byte("test"), "v2=aabb", time.Now().Unix(), "aabb", 5*time.Minute) {
		t.Error("wrong-length public key should fail")
	}
}
