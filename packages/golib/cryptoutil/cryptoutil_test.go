package cryptoutil

import (
	"encoding/hex"
	"testing"
)

func TestConstantTimeEqual(t *testing.T) {
	a := []byte("secret-key-12345")
	b := []byte("secret-key-12345")
	c := []byte("different-key!!!")

	if !ConstantTimeEqual(a, b) {
		t.Error("equal slices should match")
	}
	if ConstantTimeEqual(a, c) {
		t.Error("different slices should not match")
	}
}

func TestConstantTimeEqual_DifferentLength(t *testing.T) {
	if ConstantTimeEqual([]byte("short"), []byte("longer")) {
		t.Error("different lengths should not match")
	}
}

func TestConstantTimeEqualString(t *testing.T) {
	if !ConstantTimeEqualString("hello", "hello") {
		t.Error("equal strings should match")
	}
	if ConstantTimeEqualString("hello", "world") {
		t.Error("different strings should not match")
	}
}

func TestSecureRandom(t *testing.T) {
	b, err := SecureRandom(32)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 32 {
		t.Errorf("length: got %d", len(b))
	}

	// Two random values should differ.
	b2, _ := SecureRandom(32)
	if ConstantTimeEqual(b, b2) {
		t.Error("two random values should differ")
	}
}

func TestSecureRandom_InvalidLength(t *testing.T) {
	_, err := SecureRandom(0)
	if err == nil {
		t.Error("zero length should fail")
	}
	_, err = SecureRandom(-1)
	if err == nil {
		t.Error("negative length should fail")
	}
}

func TestSecureRandomHex(t *testing.T) {
	h, err := SecureRandomHex(16)
	if err != nil {
		t.Fatal(err)
	}
	if len(h) != 32 { // 16 bytes = 32 hex chars.
		t.Errorf("length: got %d", len(h))
	}
	// Verify it's valid hex.
	_, err = hex.DecodeString(h)
	if err != nil {
		t.Errorf("invalid hex: %v", err)
	}
}

func TestSecureRandomBase64(t *testing.T) {
	b, err := SecureRandomBase64(32)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) == 0 {
		t.Error("should produce non-empty string")
	}
}

func TestHMACSHA256(t *testing.T) {
	key := []byte("secret")
	data := []byte("message")

	mac := HMACSHA256(key, data)
	if len(mac) != 32 { // SHA-256 = 32 bytes.
		t.Errorf("HMAC length: got %d", len(mac))
	}

	// Same input should produce same output.
	mac2 := HMACSHA256(key, data)
	if !ConstantTimeEqual(mac, mac2) {
		t.Error("deterministic: same input should produce same HMAC")
	}

	// Different key should produce different output.
	mac3 := HMACSHA256([]byte("other"), data)
	if ConstantTimeEqual(mac, mac3) {
		t.Error("different keys should produce different HMACs")
	}
}

func TestHMACSHA512(t *testing.T) {
	mac := HMACSHA512([]byte("key"), []byte("data"))
	if len(mac) != 64 { // SHA-512 = 64 bytes.
		t.Errorf("HMAC-512 length: got %d", len(mac))
	}
}

func TestVerifyHMAC(t *testing.T) {
	key := []byte("secret-key")
	data := []byte("important-data")
	mac := HMACSHA256(key, data)

	if !VerifyHMAC(key, data, mac) {
		t.Error("valid HMAC should verify")
	}
	if VerifyHMAC(key, []byte("tampered"), mac) {
		t.Error("tampered data should not verify")
	}
	if VerifyHMAC([]byte("wrong-key"), data, mac) {
		t.Error("wrong key should not verify")
	}
}

func TestDeriveKey(t *testing.T) {
	master := []byte("0123456789abcdef0123456789abcdef") // 32 bytes.

	k1, err := DeriveKey(master, "encryption", 32)
	if err != nil {
		t.Fatal(err)
	}
	if len(k1) != 32 {
		t.Errorf("derived key length: got %d", len(k1))
	}

	// Different context should produce different key.
	k2, _ := DeriveKey(master, "signing", 32)
	if ConstantTimeEqual(k1, k2) {
		t.Error("different contexts should produce different keys")
	}

	// Same context should produce same key.
	k3, _ := DeriveKey(master, "encryption", 32)
	if !ConstantTimeEqual(k1, k3) {
		t.Error("same context should produce same key")
	}
}

func TestDeriveKey_ShortMaster(t *testing.T) {
	_, err := DeriveKey([]byte("short"), "ctx", 32)
	if err == nil {
		t.Error("short master key should fail")
	}
}

func TestDeriveKey_InvalidLength(t *testing.T) {
	master := []byte("0123456789abcdef")
	_, err := DeriveKey(master, "ctx", 0)
	if err == nil {
		t.Error("zero length should fail")
	}
	_, err = DeriveKey(master, "ctx", 65)
	if err == nil {
		t.Error("too long should fail")
	}
}

func TestHash(t *testing.T) {
	data := []byte("hello world")

	h256, _ := Hash("sha256", data)
	if len(h256) != 32 {
		t.Errorf("sha256: got %d bytes", len(h256))
	}

	h384, _ := Hash("sha384", data)
	if len(h384) != 48 {
		t.Errorf("sha384: got %d bytes", len(h384))
	}

	h512, _ := Hash("sha512", data)
	if len(h512) != 64 {
		t.Errorf("sha512: got %d bytes", len(h512))
	}
}

func TestHash_Unsupported(t *testing.T) {
	_, err := Hash("md5", []byte("data"))
	if err == nil {
		t.Error("md5 should not be supported")
	}
}

func TestHashHex(t *testing.T) {
	h, err := HashHex("sha256", []byte("test"))
	if err != nil {
		t.Fatal(err)
	}
	if len(h) != 64 { // 32 bytes = 64 hex chars.
		t.Errorf("hex length: got %d", len(h))
	}
}

func TestZeroBytes(t *testing.T) {
	b := []byte{1, 2, 3, 4, 5}
	ZeroBytes(b)
	for i, v := range b {
		if v != 0 {
			t.Errorf("byte %d not zeroed: %d", i, v)
		}
	}
}

func TestSecureToken(t *testing.T) {
	token, err := SecureToken(32)
	if err != nil {
		t.Fatal(err)
	}
	if len(token) == 0 {
		t.Error("token should not be empty")
	}

	token2, _ := SecureToken(32)
	if token == token2 {
		t.Error("tokens should be unique")
	}
}
