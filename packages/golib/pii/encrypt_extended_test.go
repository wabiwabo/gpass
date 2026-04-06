package pii

import (
	"strings"
	"testing"
)

func TestEncryptDecrypt_EmptyString(t *testing.T) {
	enc, err := NewEncryptor(testKey(t))
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	ciphertext, err := enc.Encrypt("")
	if err != nil {
		t.Fatalf("Encrypt empty string: %v", err)
	}

	decrypted, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt empty string: %v", err)
	}

	if decrypted != "" {
		t.Errorf("expected empty string, got %q", decrypted)
	}
}

func TestEncryptDecrypt_VeryLongString(t *testing.T) {
	enc, err := NewEncryptor(testKey(t))
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	// 10KB string
	plaintext := strings.Repeat("A", 10*1024)

	ciphertext, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt 10KB string: %v", err)
	}

	decrypted, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt 10KB string: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("10KB roundtrip failed: got length %d, want %d", len(decrypted), len(plaintext))
	}
}

func TestEncryptDecrypt_Unicode(t *testing.T) {
	enc, err := NewEncryptor(testKey(t))
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	tests := []struct {
		name      string
		plaintext string
	}{
		{"Indonesian name", "Siti Nurhaliza binti Ahmad"},
		{"Indonesian with special chars", "Jl. Sudirman No. 1, Jakarta Selatan"},
		{"Chinese characters", "张三丰"},
		{"Japanese hiragana", "こんにちは"},
		{"emoji", "Hello 😀🌍🎉"},
		{"mixed scripts", "Budi 布迪 ブディ"},
		{"Arabic", "محمد علي"},
		{"accented Latin", "Müller Göttingen Straße"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ciphertext, err := enc.Encrypt(tt.plaintext)
			if err != nil {
				t.Fatalf("Encrypt(%q): %v", tt.plaintext, err)
			}

			decrypted, err := enc.Decrypt(ciphertext)
			if err != nil {
				t.Fatalf("Decrypt: %v", err)
			}

			if decrypted != tt.plaintext {
				t.Errorf("got %q, want %q", decrypted, tt.plaintext)
			}
		})
	}
}

func TestEncryptFields_EmptyMap(t *testing.T) {
	enc, err := NewEncryptor(testKey(t))
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	result, err := enc.EncryptFields(map[string]string{})
	if err != nil {
		t.Fatalf("EncryptFields empty map: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestDecryptFields_EmptyMap(t *testing.T) {
	enc, err := NewEncryptor(testKey(t))
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	result, err := enc.DecryptFields(map[string]string{})
	if err != nil {
		t.Fatalf("DecryptFields empty map: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestMaskField_Unicode(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		lastVisible int
		want        string
	}{
		{"Indonesian name", "Siti Aminah", 3, "********nah"},
		{"Chinese name", "张三丰", 1, "**丰"},
		{"emoji", "😀😀😀😀", 2, "**😀😀"},
		{"single rune visible", "Budi", 1, "***i"},
		{"all visible", "Hi", 5, "Hi"},
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

func TestHashField_CaseSensitivity(t *testing.T) {
	enc, err := NewEncryptor(testKey(t))
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	h1 := enc.HashField("John Doe")
	h2 := enc.HashField("john doe")
	h3 := enc.HashField("JOHN DOE")

	if h1 == h2 {
		t.Error("HashField should produce different hash for different case: 'John Doe' vs 'john doe'")
	}
	if h1 == h3 {
		t.Error("HashField should produce different hash for different case: 'John Doe' vs 'JOHN DOE'")
	}
	if h2 == h3 {
		t.Error("HashField should produce different hash for different case: 'john doe' vs 'JOHN DOE'")
	}
}

func TestHashField_EmptyString(t *testing.T) {
	enc, err := NewEncryptor(testKey(t))
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	h := enc.HashField("")
	if h == "" {
		t.Error("HashField of empty string should not be empty")
	}

	// Should be deterministic
	h2 := enc.HashField("")
	if h != h2 {
		t.Error("HashField of empty string should be deterministic")
	}
}

func TestHashField_UnicodeContent(t *testing.T) {
	enc, err := NewEncryptor(testKey(t))
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	h1 := enc.HashField("Budi Santoso")
	h2 := enc.HashField("Siti Aminah")

	if h1 == h2 {
		t.Error("HashField should produce different hashes for different Indonesian names")
	}

	// Deterministic for same input
	h3 := enc.HashField("Budi Santoso")
	if h1 != h3 {
		t.Error("HashField should be deterministic for same input")
	}
}

func TestEncryptDecrypt_SpecialCharacters(t *testing.T) {
	enc, err := NewEncryptor(testKey(t))
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	tests := []struct {
		name      string
		plaintext string
	}{
		{"null bytes", "hello\x00world"},
		{"newlines", "line1\nline2\nline3"},
		{"tabs", "col1\tcol2\tcol3"},
		{"backslashes", `path\to\file`},
		{"quotes", `"quoted" and 'single'`},
		{"equals and base64 chars", "abc+def/ghi=="},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ciphertext, err := enc.Encrypt(tt.plaintext)
			if err != nil {
				t.Fatalf("Encrypt: %v", err)
			}

			decrypted, err := enc.Decrypt(ciphertext)
			if err != nil {
				t.Fatalf("Decrypt: %v", err)
			}

			if decrypted != tt.plaintext {
				t.Errorf("got %q, want %q", decrypted, tt.plaintext)
			}
		})
	}
}

func TestDecrypt_InvalidBase64(t *testing.T) {
	enc, err := NewEncryptor(testKey(t))
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	_, err = enc.Decrypt("not-valid-base64!!!")
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestDecrypt_TooShort(t *testing.T) {
	enc, err := NewEncryptor(testKey(t))
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	// Very short base64 that decodes to less than nonce size
	_, err = enc.Decrypt("AQID")
	if err == nil {
		t.Error("expected error for too-short ciphertext")
	}
}

func TestMaskField_ZeroVisible(t *testing.T) {
	got := MaskField("secret", 0)
	if got != "******" {
		t.Errorf("MaskField with 0 visible: got %q, want %q", got, "******")
	}
}

func TestEncryptFields_RoundTrip(t *testing.T) {
	enc, err := NewEncryptor(testKey(t))
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	original := map[string]string{
		"nik":     "3201234567890001",
		"name":    "Siti Nurhaliza",
		"address": "Jl. Sudirman No. 1",
		"phone":   "+6281234567890",
	}

	encrypted, err := enc.EncryptFields(original)
	if err != nil {
		t.Fatalf("EncryptFields: %v", err)
	}

	// All values should be different from originals
	for k, v := range encrypted {
		if v == original[k] {
			t.Errorf("field %q was not encrypted", k)
		}
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
