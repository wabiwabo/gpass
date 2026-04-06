package apikey

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestGenerateKey_Sandbox(t *testing.T) {
	plaintext, hash, prefix, err := GenerateKey("sandbox")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(plaintext, "gp_test_") {
		t.Errorf("sandbox key should start with gp_test_, got %s", plaintext[:16])
	}

	// Verify hash is SHA-256 of plaintext
	h := sha256.Sum256([]byte(plaintext))
	expectedHash := hex.EncodeToString(h[:])
	if hash != expectedHash {
		t.Errorf("hash mismatch: got %s, want %s", hash, expectedHash)
	}

	// Verify prefix is first 16 chars
	if prefix != plaintext[:16] {
		t.Errorf("prefix mismatch: got %s, want %s", prefix, plaintext[:16])
	}

	// Verify hash length (SHA-256 hex = 64 chars)
	if len(hash) != 64 {
		t.Errorf("hash length should be 64, got %d", len(hash))
	}
}

func TestGenerateKey_Production(t *testing.T) {
	plaintext, _, _, err := GenerateKey("production")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(plaintext, "gp_live_") {
		t.Errorf("production key should start with gp_live_, got prefix %s", plaintext[:16])
	}
}

func TestGenerateKey_InvalidEnvironment(t *testing.T) {
	_, _, _, err := GenerateKey("invalid")
	if err == nil {
		t.Fatal("expected error for invalid environment")
	}
	if !strings.Contains(err.Error(), "invalid environment") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGenerateKey_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		plaintext, _, _, err := GenerateKey("sandbox")
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if seen[plaintext] {
			t.Fatalf("duplicate key generated at iteration %d", i)
		}
		seen[plaintext] = true
	}
}

func TestGenerateKey_PrefixExtraction(t *testing.T) {
	plaintext, _, prefix, err := GenerateKey("sandbox")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Prefix should be exactly 16 chars
	if len(prefix) != 16 {
		t.Errorf("prefix length should be 16, got %d", len(prefix))
	}

	// Prefix should be extractable from plaintext
	if plaintext[:16] != prefix {
		t.Errorf("prefix does not match first 16 chars of key")
	}
}

func TestHashKey(t *testing.T) {
	key := "gp_test_abc123"
	hash := HashKey(key)
	h := sha256.Sum256([]byte(key))
	expected := hex.EncodeToString(h[:])
	if hash != expected {
		t.Errorf("HashKey mismatch: got %s, want %s", hash, expected)
	}
}

func TestEncodeBase62(t *testing.T) {
	// Encoding should only produce base62 chars
	data := []byte{0xff, 0xfe, 0xfd, 0xfc, 0xfb, 0xfa}
	encoded := encodeBase62(data)
	for _, c := range encoded {
		if !strings.ContainsRune(base62Alphabet, c) {
			t.Errorf("non-base62 character %q in encoded output", c)
		}
	}
}
