package crypto

import (
	"bytes"
	"strings"
	"testing"
)

func newFE(t *testing.T) *FieldEncryptor {
	t.Helper()
	kek := make([]byte, 32)
	for i := range kek {
		kek[i] = byte(i)
	}
	fe, err := NewFieldEncryptor(kek)
	if err != nil {
		t.Fatal(err)
	}
	return fe
}

// TestNewFieldEncryptor_RejectsBadKEKLengths pins the 32-byte length
// guard in both directions.
func TestNewFieldEncryptor_RejectsBadKEKLengths(t *testing.T) {
	for _, n := range []int{0, 16, 31, 33, 64} {
		_, err := NewFieldEncryptor(make([]byte, n))
		if err == nil {
			t.Errorf("len=%d accepted", n)
		}
	}
}

// TestEnvelope_RoundTrip pins the canonical Generate → Encrypt → Decrypt
// round-trip across multiple field shapes.
func TestEnvelope_RoundTrip(t *testing.T) {
	fe := newFE(t)
	wrapped, err := fe.GenerateWrappedDEK()
	if err != nil {
		t.Fatal(err)
	}

	cases := [][]byte{
		[]byte(""),
		[]byte("3171012345670001"),
		[]byte(strings.Repeat("x", 4096)),
	}
	for _, plain := range cases {
		ct, err := fe.EncryptField(wrapped, plain)
		if err != nil {
			t.Fatalf("encrypt %q: %v", plain, err)
		}
		got, err := fe.DecryptField(wrapped, ct)
		if err != nil {
			t.Fatalf("decrypt: %v", err)
		}
		if !bytes.Equal(got, plain) {
			t.Errorf("round-trip lost: got %q want %q", got, plain)
		}
	}
}

// TestEnvelope_NonDeterministic pins that two encryptions of the same
// plaintext under the same DEK produce different ciphertexts (random
// nonce per call). A deterministic AEAD would leak inference info.
func TestEnvelope_NonDeterministic(t *testing.T) {
	fe := newFE(t)
	wrapped, _ := fe.GenerateWrappedDEK()
	a, _ := fe.EncryptField(wrapped, []byte("same"))
	b, _ := fe.EncryptField(wrapped, []byte("same"))
	if bytes.Equal(a, b) {
		t.Error("two encryptions of identical plaintext produced identical ciphertext")
	}
}

// TestUnwrap_TooShort pins the wrapped-DEK-too-short branch via
// EncryptField (which calls unwrapDEK).
func TestUnwrap_TooShort(t *testing.T) {
	fe := newFE(t)
	if _, err := fe.EncryptField([]byte{1, 2, 3}, []byte("x")); err == nil {
		t.Error("short wrapped DEK accepted")
	}
	if _, err := fe.DecryptField([]byte{1, 2, 3}, []byte("x")); err == nil {
		t.Error("short wrapped DEK accepted in Decrypt")
	}
}

// TestUnwrap_TamperedRejected pins that flipping a byte in the wrapped
// DEK causes GCM authentication to fail.
func TestUnwrap_TamperedRejected(t *testing.T) {
	fe := newFE(t)
	wrapped, _ := fe.GenerateWrappedDEK()
	tampered := make([]byte, len(wrapped))
	copy(tampered, wrapped)
	tampered[len(tampered)-1] ^= 0xff

	if _, err := fe.EncryptField(tampered, []byte("x")); err == nil {
		t.Error("tampered wrapped DEK accepted")
	}
}

// TestDecryptField_TamperedCiphertextRejected pins the data-layer GCM
// tag check.
func TestDecryptField_TamperedCiphertextRejected(t *testing.T) {
	fe := newFE(t)
	wrapped, _ := fe.GenerateWrappedDEK()
	ct, _ := fe.EncryptField(wrapped, []byte("sensitive"))
	ct[len(ct)-1] ^= 0xff
	if _, err := fe.DecryptField(wrapped, ct); err == nil {
		t.Error("tampered ciphertext accepted")
	}
}

// TestDecryptField_TooShort pins the nonceSize guard in DecryptField.
func TestDecryptField_TooShort(t *testing.T) {
	fe := newFE(t)
	wrapped, _ := fe.GenerateWrappedDEK()
	if _, err := fe.DecryptField(wrapped, []byte{1, 2}); err == nil {
		t.Error("short ciphertext accepted")
	}
}

// TestEncryptField_DifferentDEKsProduceDifferentCiphertexts pins that
// each wrapped DEK is independent — encrypting the same plaintext under
// two different DEKs must yield ciphertexts that don't decrypt under
// the wrong DEK.
func TestEncryptField_DEKsAreIndependent(t *testing.T) {
	fe := newFE(t)
	w1, _ := fe.GenerateWrappedDEK()
	w2, _ := fe.GenerateWrappedDEK()
	if bytes.Equal(w1, w2) {
		t.Fatal("two wrapped DEKs are identical (KEK randomness broken)")
	}
	plain := []byte("payload")
	ct1, _ := fe.EncryptField(w1, plain)
	if _, err := fe.DecryptField(w2, ct1); err == nil {
		t.Error("ciphertext encrypted under DEK1 was decrypted under DEK2")
	}
}
