package crypto

import (
	"bytes"
	"testing"
)

func testKEK() []byte {
	return []byte("01234567890123456789012345678901") // 32 bytes
}

func TestFieldEncryptor_RoundTrip(t *testing.T) {
	enc, err := NewFieldEncryptor(testKEK())
	if err != nil {
		t.Fatalf("NewFieldEncryptor: %v", err)
	}
	wrappedDEK, err := enc.GenerateWrappedDEK()
	if err != nil {
		t.Fatalf("GenerateWrappedDEK: %v", err)
	}

	plaintext := []byte("Rahasia Negara")
	ct, err := enc.EncryptField(wrappedDEK, plaintext)
	if err != nil {
		t.Fatalf("EncryptField: %v", err)
	}
	decrypted, err := enc.DecryptField(wrappedDEK, ct)
	if err != nil {
		t.Fatalf("DecryptField: %v", err)
	}
	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("round trip failed: got %q, want %q", decrypted, plaintext)
	}
}

func TestFieldEncryptor_DifferentDEKs(t *testing.T) {
	enc, err := NewFieldEncryptor(testKEK())
	if err != nil {
		t.Fatalf("NewFieldEncryptor: %v", err)
	}
	dek1, _ := enc.GenerateWrappedDEK()
	dek2, _ := enc.GenerateWrappedDEK()

	if bytes.Equal(dek1, dek2) {
		t.Error("two generated DEKs should differ")
	}
}

func TestFieldEncryptor_WrongKEK(t *testing.T) {
	kek1 := []byte("01234567890123456789012345678901")
	kek2 := []byte("abcdefghijklmnopqrstuvwxyz012345")

	enc1, _ := NewFieldEncryptor(kek1)
	enc2, _ := NewFieldEncryptor(kek2)

	wrappedDEK, _ := enc1.GenerateWrappedDEK()
	ct, _ := enc1.EncryptField(wrappedDEK, []byte("secret"))

	// Try to decrypt with wrong KEK
	_, err := enc2.DecryptField(wrappedDEK, ct)
	if err == nil {
		t.Error("expected error when decrypting with wrong KEK")
	}
}

func TestFieldEncryptor_TamperedCiphertext(t *testing.T) {
	enc, _ := NewFieldEncryptor(testKEK())
	wrappedDEK, _ := enc.GenerateWrappedDEK()

	ct, _ := enc.EncryptField(wrappedDEK, []byte("secret"))
	// Tamper with the ciphertext
	ct[len(ct)-1] ^= 0xff

	_, err := enc.DecryptField(wrappedDEK, ct)
	if err == nil {
		t.Error("expected error for tampered ciphertext")
	}
}

func TestFieldEncryptor_InvalidKEKLength(t *testing.T) {
	_, err := NewFieldEncryptor([]byte("short"))
	if err == nil {
		t.Error("expected error for short KEK")
	}
}
