package pii

import (
	"crypto/rand"
	"testing"
)

func testKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return key
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	enc, err := NewEncryptor(testKey(t))
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	plaintext := "John Doe, NIK 3201234567890001"
	ciphertext, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	decrypted, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("got %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptProducesDifferentCiphertextEachTime(t *testing.T) {
	enc, err := NewEncryptor(testKey(t))
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	plaintext := "same input"
	ct1, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt 1: %v", err)
	}
	ct2, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt 2: %v", err)
	}

	if ct1 == ct2 {
		t.Error("two encryptions of the same plaintext produced identical ciphertext")
	}
}

func TestDecryptWithWrongKeyFails(t *testing.T) {
	key1 := testKey(t)
	key2 := testKey(t)

	enc1, _ := NewEncryptor(key1)
	enc2, _ := NewEncryptor(key2)

	ciphertext, err := enc1.Encrypt("secret data")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	_, err = enc2.Decrypt(ciphertext)
	if err == nil {
		t.Error("expected error decrypting with wrong key, got nil")
	}
}

func TestDecryptWithTamperedCiphertextFails(t *testing.T) {
	enc, _ := NewEncryptor(testKey(t))

	ciphertext, err := enc.Encrypt("important data")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Tamper with the ciphertext
	tampered := []byte(ciphertext)
	if len(tampered) > 5 {
		tampered[5] ^= 0xFF
	}

	_, err = enc.Decrypt(string(tampered))
	if err == nil {
		t.Error("expected error decrypting tampered ciphertext, got nil")
	}
}

func TestEncryptFields(t *testing.T) {
	enc, _ := NewEncryptor(testKey(t))

	fields := map[string]string{
		"name":  "John Doe",
		"email": "john@example.com",
		"nik":   "3201234567890001",
	}

	encrypted, err := enc.EncryptFields(fields)
	if err != nil {
		t.Fatalf("EncryptFields: %v", err)
	}

	if len(encrypted) != len(fields) {
		t.Fatalf("got %d encrypted fields, want %d", len(encrypted), len(fields))
	}

	for k, v := range encrypted {
		if v == fields[k] {
			t.Errorf("field %q was not encrypted", k)
		}
	}
}

func TestDecryptFields(t *testing.T) {
	enc, _ := NewEncryptor(testKey(t))

	original := map[string]string{
		"name":  "John Doe",
		"email": "john@example.com",
		"nik":   "3201234567890001",
	}

	encrypted, err := enc.EncryptFields(original)
	if err != nil {
		t.Fatalf("EncryptFields: %v", err)
	}

	decrypted, err := enc.DecryptFields(encrypted)
	if err != nil {
		t.Fatalf("DecryptFields: %v", err)
	}

	for k, v := range original {
		if decrypted[k] != v {
			t.Errorf("field %q: got %q, want %q", k, decrypted[k], v)
		}
	}
}

func TestMaskField(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		lastVisible int
		want        string
	}{
		{
			name:        "standard mask",
			value:       "John Doe",
			lastVisible: 3,
			want:        "*****Doe",
		},
		{
			name:        "short string all visible",
			value:       "Hi",
			lastVisible: 5,
			want:        "Hi",
		},
		{
			name:        "empty string",
			value:       "",
			lastVisible: 3,
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskField(tt.value, tt.lastVisible)
			if got != tt.want {
				t.Errorf("MaskField(%q, %d) = %q, want %q", tt.value, tt.lastVisible, got, tt.want)
			}
		})
	}
}

func TestHashFieldDeterministic(t *testing.T) {
	enc, _ := NewEncryptor(testKey(t))

	h1 := enc.HashField("john@example.com")
	h2 := enc.HashField("john@example.com")

	if h1 != h2 {
		t.Errorf("HashField not deterministic: %q != %q", h1, h2)
	}
}

func TestHashFieldDifferentInputs(t *testing.T) {
	enc, _ := NewEncryptor(testKey(t))

	h1 := enc.HashField("john@example.com")
	h2 := enc.HashField("jane@example.com")

	if h1 == h2 {
		t.Error("HashField produced same hash for different inputs")
	}
}

func TestNewEncryptorRejectsInvalidKeyLength(t *testing.T) {
	tests := []struct {
		name    string
		keyLen  int
	}{
		{"too short", 16},
		{"too long", 64},
		{"empty", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := make([]byte, tt.keyLen)
			_, err := NewEncryptor(key)
			if err == nil {
				t.Errorf("expected error for key length %d, got nil", tt.keyLen)
			}
		})
	}
}
