package config

import (
	"crypto/rand"
	"testing"
)

func testKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	return key
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := testKey(t)
	plaintext := "super-secret-value"

	encrypted, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	decrypted, err := encrypted.Decrypt(key)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestIsEncryptedTrue(t *testing.T) {
	key := testKey(t)
	encrypted, err := Encrypt("test", key)
	if err != nil {
		t.Fatal(err)
	}

	if !IsEncrypted(string(encrypted)) {
		t.Error("expected IsEncrypted to return true for encrypted value")
	}
}

func TestIsEncryptedFalse(t *testing.T) {
	tests := []string{
		"plain-text",
		"",
		"enc:v2:something",
		"not-encrypted",
	}
	for _, s := range tests {
		if IsEncrypted(s) {
			t.Errorf("expected IsEncrypted(%q) to be false", s)
		}
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	key1 := testKey(t)
	key2 := testKey(t)

	encrypted, err := Encrypt("secret", key1)
	if err != nil {
		t.Fatal(err)
	}

	_, err = encrypted.Decrypt(key2)
	if err == nil {
		t.Error("expected error when decrypting with wrong key")
	}
}

func TestDecryptEnvMixed(t *testing.T) {
	key := testKey(t)

	enc1, err := Encrypt("db-password", key)
	if err != nil {
		t.Fatal(err)
	}
	enc2, err := Encrypt("api-key-123", key)
	if err != nil {
		t.Fatal(err)
	}

	env := map[string]string{
		"DB_HOST":     "localhost",
		"DB_PORT":     "5432",
		"DB_PASSWORD": string(enc1),
		"API_KEY":     string(enc2),
	}

	result, err := DecryptEnv(env, key)
	if err != nil {
		t.Fatalf("DecryptEnv failed: %v", err)
	}

	if result["DB_HOST"] != "localhost" {
		t.Errorf("expected DB_HOST 'localhost', got %q", result["DB_HOST"])
	}
	if result["DB_PORT"] != "5432" {
		t.Errorf("expected DB_PORT '5432', got %q", result["DB_PORT"])
	}
	if result["DB_PASSWORD"] != "db-password" {
		t.Errorf("expected DB_PASSWORD 'db-password', got %q", result["DB_PASSWORD"])
	}
	if result["API_KEY"] != "api-key-123" {
		t.Errorf("expected API_KEY 'api-key-123', got %q", result["API_KEY"])
	}
}

func TestDifferentEncryptionsProduceDifferentCiphertext(t *testing.T) {
	key := testKey(t)
	plaintext := "same-value"

	enc1, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatal(err)
	}
	enc2, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatal(err)
	}

	if enc1 == enc2 {
		t.Error("expected different ciphertexts for same plaintext (random nonce)")
	}

	// Both should still decrypt to the same value.
	dec1, _ := enc1.Decrypt(key)
	dec2, _ := enc2.Decrypt(key)
	if dec1 != dec2 {
		t.Errorf("decrypted values differ: %q vs %q", dec1, dec2)
	}
}

func TestEncryptInvalidKeyLength(t *testing.T) {
	_, err := Encrypt("test", []byte("short"))
	if err == nil {
		t.Error("expected error for short key")
	}
}
