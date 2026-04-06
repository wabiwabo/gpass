package cryptorand

import (
	"encoding/hex"
	"strings"
	"testing"
	"unicode"
)

func TestBytes(t *testing.T) {
	b, err := Bytes(32)
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	if len(b) != 32 {
		t.Errorf("len = %d, want 32", len(b))
	}

	// Should be random (not all zeros)
	allZero := true
	for _, v := range b {
		if v != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("bytes should not all be zero")
	}
}

func TestBytes_Uniqueness(t *testing.T) {
	b1, _ := Bytes(32)
	b2, _ := Bytes(32)
	if string(b1) == string(b2) {
		t.Error("two random byte slices should not be identical")
	}
}

func TestHex(t *testing.T) {
	s, err := Hex(16)
	if err != nil {
		t.Fatalf("Hex: %v", err)
	}
	if len(s) != 32 {
		t.Errorf("len = %d, want 32", len(s))
	}
	if _, err := hex.DecodeString(s); err != nil {
		t.Errorf("not valid hex: %v", err)
	}
}

func TestBase64(t *testing.T) {
	s, err := Base64(32)
	if err != nil {
		t.Fatalf("Base64: %v", err)
	}
	if len(s) == 0 {
		t.Error("should not be empty")
	}
	// URL-safe base64 should not contain + or /
	if strings.ContainsAny(s, "+/") {
		t.Errorf("should be URL-safe: %q", s)
	}
}

func TestBase64Raw(t *testing.T) {
	s, err := Base64Raw(32)
	if err != nil {
		t.Fatalf("Base64Raw: %v", err)
	}
	if strings.ContainsRune(s, '=') {
		t.Errorf("should not have padding: %q", s)
	}
}

func TestToken(t *testing.T) {
	tok, err := Token()
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	// 32 bytes → 64 hex chars
	if len(tok) != 64 {
		t.Errorf("len = %d, want 64", len(tok))
	}
	if _, err := hex.DecodeString(tok); err != nil {
		t.Errorf("not valid hex: %v", err)
	}
}

func TestToken_Unique(t *testing.T) {
	tokens := make(map[string]bool)
	for i := 0; i < 100; i++ {
		tok, _ := Token()
		if tokens[tok] {
			t.Fatal("duplicate token generated")
		}
		tokens[tok] = true
	}
}

func TestOTP(t *testing.T) {
	tests := []struct {
		name   string
		length int
		wantOK bool
	}{
		{"4 digits", 4, true},
		{"6 digits", 6, true},
		{"8 digits", 8, true},
		{"10 digits", 10, true},
		{"too short", 3, false},
		{"too long", 11, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			otp, err := OTP(tt.length)
			if tt.wantOK {
				if err != nil {
					t.Fatalf("OTP(%d): %v", tt.length, err)
				}
				if len(otp) != tt.length {
					t.Errorf("len = %d, want %d", len(otp), tt.length)
				}
				for _, c := range otp {
					if !unicode.IsDigit(c) {
						t.Errorf("non-digit character: %c", c)
					}
				}
			} else {
				if err == nil {
					t.Error("expected error")
				}
			}
		})
	}
}

func TestOTP_LeadingZeros(t *testing.T) {
	// Run many times to verify zero-padding works
	for i := 0; i < 100; i++ {
		otp, err := OTP(6)
		if err != nil {
			t.Fatal(err)
		}
		if len(otp) != 6 {
			t.Errorf("OTP length = %d, got %q", len(otp), otp)
		}
	}
}

func TestID(t *testing.T) {
	id, err := ID(16)
	if err != nil {
		t.Fatalf("ID: %v", err)
	}
	if len(id) == 0 {
		t.Error("ID should not be empty")
	}
	// URL-safe
	if strings.ContainsAny(id, "+/=") {
		t.Errorf("ID should be URL-safe: %q", id)
	}
}

func TestID_DefaultEntropy(t *testing.T) {
	id, err := ID(0)
	if err != nil {
		t.Fatalf("ID: %v", err)
	}
	if len(id) == 0 {
		t.Error("should use default entropy")
	}
}

func TestChoose(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e"}
	chosen, err := Choose(items)
	if err != nil {
		t.Fatalf("Choose: %v", err)
	}

	found := false
	for _, item := range items {
		if chosen == item {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("chose %q which is not in items", chosen)
	}
}

func TestChoose_EmptySlice(t *testing.T) {
	_, err := Choose([]string{})
	if err == nil {
		t.Error("expected error for empty slice")
	}
}

func TestChoose_SingleItem(t *testing.T) {
	chosen, err := Choose([]int{42})
	if err != nil {
		t.Fatal(err)
	}
	if chosen != 42 {
		t.Errorf("chose %d, want 42", chosen)
	}
}

func TestShuffle(t *testing.T) {
	items := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	original := make([]int, len(items))
	copy(original, items)

	if err := Shuffle(items); err != nil {
		t.Fatalf("Shuffle: %v", err)
	}

	// Same length
	if len(items) != len(original) {
		t.Fatal("length changed")
	}

	// Contains same elements
	sum := 0
	for _, v := range items {
		sum += v
	}
	if sum != 55 {
		t.Error("elements changed during shuffle")
	}
}

func TestShuffle_Empty(t *testing.T) {
	var items []int
	if err := Shuffle(items); err != nil {
		t.Errorf("should handle empty slice: %v", err)
	}
}

func TestShuffle_SingleItem(t *testing.T) {
	items := []int{1}
	if err := Shuffle(items); err != nil {
		t.Errorf("should handle single item: %v", err)
	}
	if items[0] != 1 {
		t.Error("single item changed")
	}
}

func TestPassword(t *testing.T) {
	pw, err := Password(16)
	if err != nil {
		t.Fatalf("Password: %v", err)
	}
	if len(pw) != 16 {
		t.Errorf("len = %d, want 16", len(pw))
	}

	// Check character classes
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, c := range pw {
		switch {
		case unicode.IsUpper(c):
			hasUpper = true
		case unicode.IsLower(c):
			hasLower = true
		case unicode.IsDigit(c):
			hasDigit = true
		default:
			hasSpecial = true
		}
	}

	if !hasUpper {
		t.Error("password missing uppercase")
	}
	if !hasLower {
		t.Error("password missing lowercase")
	}
	if !hasDigit {
		t.Error("password missing digit")
	}
	if !hasSpecial {
		t.Error("password missing special character")
	}
}

func TestPassword_TooShort(t *testing.T) {
	_, err := Password(7)
	if err == nil {
		t.Error("should reject length < 8")
	}
}

func TestPassword_MinLength(t *testing.T) {
	pw, err := Password(8)
	if err != nil {
		t.Fatalf("Password(8): %v", err)
	}
	if len(pw) != 8 {
		t.Errorf("len = %d", len(pw))
	}
}

func TestPassword_Unique(t *testing.T) {
	passwords := make(map[string]bool)
	for i := 0; i < 50; i++ {
		pw, _ := Password(16)
		if passwords[pw] {
			t.Fatal("duplicate password")
		}
		passwords[pw] = true
	}
}
