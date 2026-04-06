package crypto

import (
	"testing"
)

func TestHMACSHA256Consistent(t *testing.T) {
	message := []byte("hello world")
	key := []byte("secret-key")

	h1 := HMACSHA256(message, key)
	h2 := HMACSHA256(message, key)

	if h1 != h2 {
		t.Errorf("HMACSHA256 not consistent: %q != %q", h1, h2)
	}

	// SHA256 HMAC produces 32 bytes = 64 hex chars.
	if len(h1) != 64 {
		t.Errorf("HMACSHA256 length = %d, want 64", len(h1))
	}
}

func TestHMACSHA256DifferentKeys(t *testing.T) {
	message := []byte("hello world")

	h1 := HMACSHA256(message, []byte("key1"))
	h2 := HMACSHA256(message, []byte("key2"))

	if h1 == h2 {
		t.Error("different keys should produce different HMACs")
	}
}

func TestHMACSHA256DifferentMessages(t *testing.T) {
	key := []byte("secret-key")

	h1 := HMACSHA256([]byte("message1"), key)
	h2 := HMACSHA256([]byte("message2"), key)

	if h1 == h2 {
		t.Error("different messages should produce different HMACs")
	}
}

func TestVerifyHMACSHA256Matching(t *testing.T) {
	message := []byte("hello world")
	key := []byte("secret-key")

	mac := HMACSHA256(message, key)
	if !VerifyHMACSHA256(message, key, mac) {
		t.Error("VerifyHMACSHA256 should return true for matching HMAC")
	}
}

func TestVerifyHMACSHA256TamperedMessage(t *testing.T) {
	key := []byte("secret-key")
	mac := HMACSHA256([]byte("original"), key)

	if VerifyHMACSHA256([]byte("tampered"), key, mac) {
		t.Error("VerifyHMACSHA256 should return false for tampered message")
	}
}

func TestVerifyHMACSHA256WrongKey(t *testing.T) {
	message := []byte("hello world")
	mac := HMACSHA256(message, []byte("key1"))

	if VerifyHMACSHA256(message, []byte("key2"), mac) {
		t.Error("VerifyHMACSHA256 should return false for wrong key")
	}
}

func TestVerifyHMACSHA256InvalidHex(t *testing.T) {
	if VerifyHMACSHA256([]byte("msg"), []byte("key"), "not-hex") {
		t.Error("VerifyHMACSHA256 should return false for invalid hex")
	}
}

func TestGenerateRandomHexLength(t *testing.T) {
	tests := []struct {
		n       int
		wantLen int
	}{
		{16, 32},
		{32, 64},
		{1, 2},
	}

	for _, tt := range tests {
		h, err := GenerateRandomHex(tt.n)
		if err != nil {
			t.Fatalf("GenerateRandomHex(%d): %v", tt.n, err)
		}
		if len(h) != tt.wantLen {
			t.Errorf("GenerateRandomHex(%d) len = %d, want %d", tt.n, len(h), tt.wantLen)
		}
	}
}

func TestGenerateRandomBytesUnique(t *testing.T) {
	b1, err := GenerateRandomBytes(32)
	if err != nil {
		t.Fatalf("GenerateRandomBytes: %v", err)
	}
	b2, err := GenerateRandomBytes(32)
	if err != nil {
		t.Fatalf("GenerateRandomBytes: %v", err)
	}

	if string(b1) == string(b2) {
		t.Error("two random byte slices should not be equal")
	}
}

func TestGenerateRandomBytesRejectsZero(t *testing.T) {
	_, err := GenerateRandomBytes(0)
	if err == nil {
		t.Error("expected error for n=0")
	}
}

func TestGenerateRandomBytesRejectsNegative(t *testing.T) {
	_, err := GenerateRandomBytes(-1)
	if err == nil {
		t.Error("expected error for n=-1")
	}
}
