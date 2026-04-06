package hmacverify

import "testing"

var secret = []byte("test-secret-key-32bytes-long!!!!")

func TestSign_Deterministic(t *testing.T) {
	s1 := Sign(secret, SHA256, ":", "method", "path", "timestamp")
	s2 := Sign(secret, SHA256, ":", "method", "path", "timestamp")
	if s1 != s2 {
		t.Error("should be deterministic")
	}
}

func TestSign_DifferentParts(t *testing.T) {
	s1 := Sign(secret, SHA256, ":", "GET", "/api")
	s2 := Sign(secret, SHA256, ":", "POST", "/api")
	if s1 == s2 {
		t.Error("different parts should produce different signatures")
	}
}

func TestSign_DifferentSecrets(t *testing.T) {
	s1 := Sign([]byte("secret1"), SHA256, ":", "data")
	s2 := Sign([]byte("secret2"), SHA256, ":", "data")
	if s1 == s2 {
		t.Error("different secrets should differ")
	}
}

func TestVerify_Valid(t *testing.T) {
	sig := Sign(secret, SHA256, ":", "GET", "/users", "12345")
	if !Verify(secret, SHA256, ":", sig, "GET", "/users", "12345") {
		t.Error("valid signature should verify")
	}
}

func TestVerify_Invalid(t *testing.T) {
	if Verify(secret, SHA256, ":", "bad-signature", "GET", "/users") {
		t.Error("invalid signature should not verify")
	}
}

func TestVerify_Tampered(t *testing.T) {
	sig := Sign(secret, SHA256, ":", "GET", "/users")
	if Verify(secret, SHA256, ":", sig, "POST", "/users") {
		t.Error("tampered parts should not verify")
	}
}

func TestSignBytes(t *testing.T) {
	sig := SignBytes(secret, SHA256, []byte("payload data"))
	if sig == "" {
		t.Error("should not be empty")
	}
	if len(sig) != 64 { // SHA-256 hex
		t.Errorf("len = %d, want 64", len(sig))
	}
}

func TestVerifyBytes_Valid(t *testing.T) {
	data := []byte(`{"user":"test"}`)
	sig := SignBytes(secret, SHA256, data)
	if !VerifyBytes(secret, SHA256, data, sig) {
		t.Error("valid should verify")
	}
}

func TestVerifyBytes_Invalid(t *testing.T) {
	if VerifyBytes(secret, SHA256, []byte("data"), "wrong") {
		t.Error("invalid should not verify")
	}
}

func TestSHA512(t *testing.T) {
	sig256 := Sign(secret, SHA256, ":", "data")
	sig512 := Sign(secret, SHA512, ":", "data")
	if sig256 == sig512 {
		t.Error("different algorithms should differ")
	}
	if len(sig512) != 128 { // SHA-512 hex
		t.Errorf("SHA512 len = %d, want 128", len(sig512))
	}
}

func TestVerify_SHA512(t *testing.T) {
	sig := Sign(secret, SHA512, ".", "a", "b", "c")
	if !Verify(secret, SHA512, ".", sig, "a", "b", "c") {
		t.Error("SHA512 should verify")
	}
}

func TestSign_CustomSeparator(t *testing.T) {
	s1 := Sign(secret, SHA256, "\n", "line1", "line2")
	s2 := Sign(secret, SHA256, ":", "line1", "line2")
	if s1 == s2 {
		t.Error("different separators should differ")
	}
}
