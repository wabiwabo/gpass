package cryptohash

import (
	"encoding/hex"
	"testing"
)

func TestSHA256(t *testing.T) {
	// Known test vector
	got := SHA256([]byte("hello"))
	want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSHA256String(t *testing.T) {
	got := SHA256String("hello")
	want := SHA256([]byte("hello"))
	if got != want {
		t.Error("SHA256String should equal SHA256([]byte)")
	}
}

func TestSHA256Empty(t *testing.T) {
	got := SHA256([]byte{})
	// SHA256 of empty string
	want := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSHA512Hex(t *testing.T) {
	got := SHA512Hex([]byte("hello"))
	if len(got) != 128 { // 64 bytes = 128 hex chars
		t.Errorf("SHA512 hex length: got %d, want 128", len(got))
	}
}

func TestSHA256Bytes(t *testing.T) {
	got := SHA256Bytes([]byte("hello"))
	if len(got) != 32 {
		t.Errorf("SHA256 bytes length: got %d, want 32", len(got))
	}
	hexStr := hex.EncodeToString(got)
	if hexStr != SHA256([]byte("hello")) {
		t.Error("bytes and hex should match")
	}
}

func TestHMAC256(t *testing.T) {
	key := []byte("secret-key")
	data := []byte("message")
	got := HMAC256(key, data)
	if len(got) != 64 { // 32 bytes = 64 hex chars
		t.Errorf("HMAC256 length: got %d, want 64", len(got))
	}
}

func TestHMAC256Deterministic(t *testing.T) {
	key := []byte("key")
	data := []byte("data")
	h1 := HMAC256(key, data)
	h2 := HMAC256(key, data)
	if h1 != h2 {
		t.Error("HMAC should be deterministic")
	}
}

func TestHMAC256DifferentKeys(t *testing.T) {
	data := []byte("same data")
	h1 := HMAC256([]byte("key1"), data)
	h2 := HMAC256([]byte("key2"), data)
	if h1 == h2 {
		t.Error("different keys should produce different HMACs")
	}
}

func TestHMAC256Bytes(t *testing.T) {
	key := []byte("key")
	data := []byte("data")
	got := HMAC256Bytes(key, data)
	if len(got) != 32 {
		t.Errorf("length: got %d, want 32", len(got))
	}
}

func TestHMAC512(t *testing.T) {
	got := HMAC512([]byte("key"), []byte("data"))
	if len(got) != 128 { // 64 bytes = 128 hex chars
		t.Errorf("HMAC512 length: got %d, want 128", len(got))
	}
}

func TestVerifyHMAC256(t *testing.T) {
	key := []byte("secret")
	data := []byte("payload")
	sig := HMAC256(key, data)

	if !VerifyHMAC256(key, data, sig) {
		t.Error("valid HMAC should verify")
	}
	if VerifyHMAC256(key, data, "invalid-hex") {
		t.Error("invalid hex should not verify")
	}
	if VerifyHMAC256(key, data, HMAC256([]byte("wrong-key"), data)) {
		t.Error("wrong key should not verify")
	}
	if VerifyHMAC256(key, []byte("wrong-data"), sig) {
		t.Error("wrong data should not verify")
	}
}

func TestVerifyHMAC256ConstantTime(t *testing.T) {
	key := []byte("secret")
	data := []byte("payload")
	sig := HMAC256(key, data)
	// Modify one character
	badSig := sig[:len(sig)-1] + "0"
	if VerifyHMAC256(key, data, badSig) {
		t.Error("modified signature should not verify")
	}
}
