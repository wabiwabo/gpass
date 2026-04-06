package pem

import (
	"testing"
)

func makeCert() []byte {
	return Encode(TypeCertificate, []byte("fake cert data for testing"))
}

func makeKey() []byte {
	return Encode(TypeECPrivateKey, []byte("fake key data for testing"))
}

func TestParse(t *testing.T) {
	block, err := Parse(makeCert())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if block.Type != TypeCertificate {
		t.Errorf("Type = %q", block.Type)
	}
	if len(block.Bytes) == 0 {
		t.Error("Bytes should not be empty")
	}
	if string(block.Bytes) != "fake cert data for testing" {
		t.Errorf("Bytes = %q", block.Bytes)
	}
}

func TestParse_Invalid(t *testing.T) {
	_, err := Parse([]byte("not pem data"))
	if err == nil {
		t.Error("should error on invalid PEM")
	}
}

func TestParseAll(t *testing.T) {
	combined := append(makeCert(), '\n')
	combined = append(combined, makeKey()...)

	blocks, err := ParseAll(combined)
	if err != nil {
		t.Fatalf("ParseAll: %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("blocks = %d, want 2", len(blocks))
	}
	if blocks[0].Type != TypeCertificate {
		t.Errorf("[0].Type = %q", blocks[0].Type)
	}
	if blocks[1].Type != TypeECPrivateKey {
		t.Errorf("[1].Type = %q", blocks[1].Type)
	}
}

func TestParseAll_Invalid(t *testing.T) {
	_, err := ParseAll([]byte("not pem"))
	if err == nil {
		t.Error("should error")
	}
}

func TestEncode(t *testing.T) {
	data := []byte("test data")
	encoded := Encode("TEST BLOCK", data)

	if !IsPEM(encoded) {
		t.Error("should be valid PEM")
	}

	block, err := Parse(encoded)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if block.Type != "TEST BLOCK" {
		t.Errorf("Type = %q", block.Type)
	}
	if string(block.Bytes) != "test data" {
		t.Errorf("Bytes = %q", block.Bytes)
	}
}

func TestEncodeWithHeaders(t *testing.T) {
	headers := map[string]string{"Key-ID": "abc123"}
	encoded := EncodeWithHeaders("PUBLIC KEY", []byte("key data"), headers)

	block, _ := Parse(encoded)
	if block.Headers["Key-ID"] != "abc123" {
		t.Errorf("headers = %v", block.Headers)
	}
}

func TestIsPEM(t *testing.T) {
	if !IsPEM(makeCert()) {
		t.Error("cert should be PEM")
	}
	if IsPEM([]byte("not pem")) {
		t.Error("plain text should not be PEM")
	}
}

func TestTypeOf(t *testing.T) {
	if TypeOf(makeCert()) != TypeCertificate {
		t.Errorf("TypeOf(cert) = %q", TypeOf(makeCert()))
	}
	if TypeOf(makeKey()) != TypeECPrivateKey {
		t.Errorf("TypeOf(key) = %q", TypeOf(makeKey()))
	}
	if TypeOf([]byte("invalid")) != "" {
		t.Error("invalid should return empty")
	}
}

func TestIsCertificate(t *testing.T) {
	if !IsCertificate(makeCert()) {
		t.Error("should be certificate")
	}
	if IsCertificate(makeKey()) {
		t.Error("key should not be certificate")
	}
}

func TestIsPrivateKey(t *testing.T) {
	if !IsPrivateKey(makeKey()) {
		t.Error("should be private key")
	}
	if IsPrivateKey(makeCert()) {
		t.Error("cert should not be private key")
	}

	// Also test other private key types
	rsaKey := Encode(TypeRSAPrivateKey, []byte("rsa"))
	if !IsPrivateKey(rsaKey) {
		t.Error("RSA key should be private key")
	}
	genericKey := Encode(TypePrivateKey, []byte("generic"))
	if !IsPrivateKey(genericKey) {
		t.Error("generic PRIVATE KEY should be private key")
	}
}

func TestCount(t *testing.T) {
	if Count(makeCert()) != 1 {
		t.Errorf("single cert count = %d", Count(makeCert()))
	}

	combined := append(makeCert(), '\n')
	combined = append(combined, makeKey()...)
	if Count(combined) != 2 {
		t.Errorf("combined count = %d", Count(combined))
	}

	if Count([]byte("not pem")) != 0 {
		t.Error("invalid should be 0")
	}
}

func TestConstants(t *testing.T) {
	if TypeCertificate != "CERTIFICATE" {
		t.Error("TypeCertificate constant")
	}
	if TypePrivateKey != "PRIVATE KEY" {
		t.Error("TypePrivateKey constant")
	}
	if TypeRSAPrivateKey != "RSA PRIVATE KEY" {
		t.Error("TypeRSAPrivateKey constant")
	}
	if TypeECPrivateKey != "EC PRIVATE KEY" {
		t.Error("TypeECPrivateKey constant")
	}
	if TypePublicKey != "PUBLIC KEY" {
		t.Error("TypePublicKey constant")
	}
	if TypeCSR != "CERTIFICATE REQUEST" {
		t.Error("TypeCSR constant")
	}
}

func TestRoundTrip(t *testing.T) {
	original := []byte("important certificate data")
	encoded := Encode(TypeCertificate, original)
	block, err := Parse(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if string(block.Bytes) != string(original) {
		t.Error("roundtrip failed")
	}
}
