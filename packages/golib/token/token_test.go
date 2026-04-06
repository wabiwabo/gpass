package token

import (
	"strings"
	"testing"
)

func TestGenerate_Hex(t *testing.T) {
	tok, err := Generate(32)
	if err != nil {
		t.Fatal(err)
	}
	if len(tok) != 64 {
		t.Errorf("expected 64 hex chars, got %d", len(tok))
	}
}

func TestGenerate_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		tok, err := Generate(32)
		if err != nil {
			t.Fatal(err)
		}
		if seen[tok] {
			t.Fatal("duplicate token")
		}
		seen[tok] = true
	}
}

func TestGenerateBase64URL(t *testing.T) {
	tok, err := GenerateBase64URL(32)
	if err != nil {
		t.Fatal(err)
	}
	// base64url of 32 bytes = 43 chars (no padding).
	if len(tok) != 43 {
		t.Errorf("expected 43 chars, got %d: %q", len(tok), tok)
	}
	if strings.ContainsAny(tok, "+/=") {
		t.Error("base64url should not contain +, /, or =")
	}
}

func TestGenerateBase62(t *testing.T) {
	tok, err := GenerateBase62(32)
	if err != nil {
		t.Fatal(err)
	}
	if len(tok) != 32 {
		t.Errorf("expected 32 chars, got %d", len(tok))
	}
	for _, c := range tok {
		if !strings.ContainsRune(base62Alphabet, c) {
			t.Errorf("invalid base62 char: %c", c)
		}
	}
}

func TestGenerateBase62_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		tok, _ := GenerateBase62(32)
		if seen[tok] {
			t.Fatal("duplicate base62 token")
		}
		seen[tok] = true
	}
}

func TestGenerateUUID(t *testing.T) {
	uuid, err := GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}

	// Check format: 8-4-4-4-12.
	parts := strings.Split(uuid, "-")
	if len(parts) != 5 {
		t.Fatalf("expected 5 parts, got %d: %q", len(parts), uuid)
	}
	if len(parts[0]) != 8 || len(parts[1]) != 4 || len(parts[2]) != 4 || len(parts[3]) != 4 || len(parts[4]) != 12 {
		t.Errorf("invalid UUID format: %q", uuid)
	}

	// Version 4: 3rd group starts with '4'.
	if parts[2][0] != '4' {
		t.Errorf("UUID version should be 4, got %c", parts[2][0])
	}

	// Variant: 4th group starts with 8, 9, a, or b.
	first := parts[3][0]
	if first != '8' && first != '9' && first != 'a' && first != 'b' {
		t.Errorf("UUID variant byte should be 8/9/a/b, got %c", first)
	}
}

func TestGenerateUUID_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		uuid, _ := GenerateUUID()
		if seen[uuid] {
			t.Fatal("duplicate UUID")
		}
		seen[uuid] = true
	}
}

func TestGenerateAPIKey_Live(t *testing.T) {
	key, err := GenerateAPIKey("live", 32)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(key, "gp_live_") {
		t.Errorf("expected prefix gp_live_, got %q", key)
	}
	// Total: "gp_live_" (8) + 32 = 40.
	if len(key) != 40 {
		t.Errorf("expected 40 chars, got %d", len(key))
	}
}

func TestGenerateAPIKey_Test(t *testing.T) {
	key, err := GenerateAPIKey("test", 32)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(key, "gp_test_") {
		t.Errorf("expected prefix gp_test_, got %q", key)
	}
}

func TestGenerateAPIKey_InvalidEnv(t *testing.T) {
	_, err := GenerateAPIKey("staging", 32)
	if err == nil {
		t.Error("should reject invalid env")
	}
}

func TestGenerateAPIKey_MinLength(t *testing.T) {
	key, err := GenerateAPIKey("live", 5) // below minimum
	if err != nil {
		t.Fatal(err)
	}
	// Should use default 32.
	if !strings.HasPrefix(key, "gp_live_") {
		t.Error("wrong prefix")
	}
	tokenPart := strings.TrimPrefix(key, "gp_live_")
	if len(tokenPart) != 32 {
		t.Errorf("token part should default to 32, got %d", len(tokenPart))
	}
}

func TestGenerateOTP_6Digits(t *testing.T) {
	otp, err := GenerateOTP(6)
	if err != nil {
		t.Fatal(err)
	}
	if len(otp) != 6 {
		t.Errorf("expected 6 digits, got %d: %q", len(otp), otp)
	}
	for _, c := range otp {
		if c < '0' || c > '9' {
			t.Errorf("non-digit in OTP: %c", c)
		}
	}
}

func TestGenerateOTP_4Digits(t *testing.T) {
	otp, err := GenerateOTP(4)
	if err != nil {
		t.Fatal(err)
	}
	if len(otp) != 4 {
		t.Errorf("expected 4 digits, got %d", len(otp))
	}
}

func TestGenerateOTP_InvalidDigits(t *testing.T) {
	_, err := GenerateOTP(3)
	if err == nil {
		t.Error("should reject < 4 digits")
	}
	_, err = GenerateOTP(11)
	if err == nil {
		t.Error("should reject > 10 digits")
	}
}

func TestBytes(t *testing.T) {
	b, err := Bytes(32)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 32 {
		t.Errorf("expected 32 bytes, got %d", len(b))
	}
}
