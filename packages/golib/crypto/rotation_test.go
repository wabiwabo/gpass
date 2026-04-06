package crypto

import (
	"crypto/rand"
	"fmt"
	"testing"
)

func generateKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	return key
}

func TestKeyRing_EncryptDecrypt(t *testing.T) {
	key := generateKey(t)
	kr, err := NewKeyRing(key)
	if err != nil {
		t.Fatal(err)
	}

	plaintext := "sensitive PII data: NIK 3201234567890001"
	ciphertext, err := kr.Encrypt(plaintext)
	if err != nil {
		t.Fatal(err)
	}

	if ciphertext == plaintext {
		t.Fatal("ciphertext should differ from plaintext")
	}

	decrypted, err := kr.Decrypt(ciphertext)
	if err != nil {
		t.Fatal(err)
	}

	if decrypted != plaintext {
		t.Fatalf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestKeyRing_RotateAndEncryptWithNewKey(t *testing.T) {
	key1 := generateKey(t)
	kr, err := NewKeyRing(key1)
	if err != nil {
		t.Fatal(err)
	}

	if kr.ActiveVersion() != 1 {
		t.Fatalf("expected active version 1, got %d", kr.ActiveVersion())
	}

	key2 := generateKey(t)
	if err := kr.Rotate(key2); err != nil {
		t.Fatal(err)
	}

	if kr.ActiveVersion() != 2 {
		t.Fatalf("expected active version 2, got %d", kr.ActiveVersion())
	}

	plaintext := "data encrypted after rotation"
	ciphertext, err := kr.Encrypt(plaintext)
	if err != nil {
		t.Fatal(err)
	}

	decrypted, err := kr.Decrypt(ciphertext)
	if err != nil {
		t.Fatal(err)
	}

	if decrypted != plaintext {
		t.Fatalf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestKeyRing_DecryptOldDataAfterRotation(t *testing.T) {
	key1 := generateKey(t)
	kr, err := NewKeyRing(key1)
	if err != nil {
		t.Fatal(err)
	}

	plaintext := "data encrypted with key v1"
	ciphertext, err := kr.Encrypt(plaintext)
	if err != nil {
		t.Fatal(err)
	}

	// Rotate to a new key
	key2 := generateKey(t)
	if err := kr.Rotate(key2); err != nil {
		t.Fatal(err)
	}

	// Old data should still decrypt
	decrypted, err := kr.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("failed to decrypt old data after rotation: %v", err)
	}

	if decrypted != plaintext {
		t.Fatalf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestKeyRing_ActiveVersionChangesAfterRotation(t *testing.T) {
	key1 := generateKey(t)
	kr, err := NewKeyRing(key1)
	if err != nil {
		t.Fatal(err)
	}

	if v := kr.ActiveVersion(); v != 1 {
		t.Fatalf("expected version 1, got %d", v)
	}

	kr.Rotate(generateKey(t))
	if v := kr.ActiveVersion(); v != 2 {
		t.Fatalf("expected version 2, got %d", v)
	}

	kr.Rotate(generateKey(t))
	if v := kr.ActiveVersion(); v != 3 {
		t.Fatalf("expected version 3, got %d", v)
	}
}

func TestKeyRing_VersionsReturnsAll(t *testing.T) {
	key1 := generateKey(t)
	kr, err := NewKeyRing(key1)
	if err != nil {
		t.Fatal(err)
	}

	kr.Rotate(generateKey(t))
	kr.Rotate(generateKey(t))

	versions := kr.Versions()
	if len(versions) != 3 {
		t.Fatalf("expected 3 versions, got %d", len(versions))
	}

	// Only the last should be active
	activeCount := 0
	for _, v := range versions {
		if v.Active {
			activeCount++
			if v.Version != 3 {
				t.Errorf("expected active version 3, got %d", v.Version)
			}
		}
	}
	if activeCount != 1 {
		t.Fatalf("expected exactly 1 active key, got %d", activeCount)
	}

	// Versions should be 1, 2, 3
	for i, v := range versions {
		if v.Version != i+1 {
			t.Errorf("expected version %d at index %d, got %d", i+1, i, v.Version)
		}
	}
}

func TestKeyRing_InvalidKeySize(t *testing.T) {
	tests := []struct {
		name    string
		keySize int
	}{
		{"too short", 16},
		{"too long", 64},
		{"empty", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := make([]byte, tt.keySize)
			_, err := NewKeyRing(key)
			if err == nil {
				t.Fatal("expected error for invalid key size")
			}
		})
	}
}

func TestKeyRing_RotateInvalidKeySize(t *testing.T) {
	kr, err := NewKeyRing(generateKey(t))
	if err != nil {
		t.Fatal(err)
	}

	err = kr.Rotate(make([]byte, 16))
	if err == nil {
		t.Fatal("expected error for invalid rotation key size")
	}
}

func TestKeyRing_MultipleRotationsPreserveAllKeys(t *testing.T) {
	kr, err := NewKeyRing(generateKey(t))
	if err != nil {
		t.Fatal(err)
	}

	// Encrypt with each version and store ciphertexts
	ciphertexts := make([]string, 5)
	plaintexts := make([]string, 5)

	for i := 0; i < 5; i++ {
		plaintexts[i] = fmt.Sprintf("data for version %d", i+1)
		ct, err := kr.Encrypt(plaintexts[i])
		if err != nil {
			t.Fatalf("encrypt v%d: %v", i+1, err)
		}
		ciphertexts[i] = ct

		if i < 4 { // don't rotate after the last one
			if err := kr.Rotate(generateKey(t)); err != nil {
				t.Fatalf("rotate %d: %v", i+1, err)
			}
		}
	}

	// All ciphertexts should still decrypt
	for i, ct := range ciphertexts {
		decrypted, err := kr.Decrypt(ct)
		if err != nil {
			t.Fatalf("decrypt v%d data: %v", i+1, err)
		}
		if decrypted != plaintexts[i] {
			t.Errorf("v%d: expected %q, got %q", i+1, plaintexts[i], decrypted)
		}
	}

	if v := kr.ActiveVersion(); v != 5 {
		t.Fatalf("expected active version 5, got %d", v)
	}

	versions := kr.Versions()
	if len(versions) != 5 {
		t.Fatalf("expected 5 versions, got %d", len(versions))
	}
}

func TestKeyRing_EmptyPlaintext(t *testing.T) {
	kr, err := NewKeyRing(generateKey(t))
	if err != nil {
		t.Fatal(err)
	}

	ct, err := kr.Encrypt("")
	if err != nil {
		t.Fatal(err)
	}

	pt, err := kr.Decrypt(ct)
	if err != nil {
		t.Fatal(err)
	}

	if pt != "" {
		t.Fatalf("expected empty string, got %q", pt)
	}
}
