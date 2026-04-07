package crypto

import (
	"strings"
	"testing"
)

func TestValidateNIKFormatEdgeCases(t *testing.T) {
	tests := []struct {
		name string
		nik  string
		ok   bool
	}{
		{"valid_jakarta", "3201012345678901", true},
		{"valid_papua", "9101012345678901", true},
		{"valid_aceh", "1101012345678901", true},
		{"valid_bali", "5101012345678901", true},
		{"too_short", "123456789", false},
		{"too_long", "12345678901234567", false},
		{"empty", "", false},
		{"contains_letters", "32010A2345678901", false},
		{"contains_dash", "32-01012345678901", false},
		{"contains_spaces", "3201 0123456789 ", false},
		{"province_code_00", "0001012345678901", false},
		{"province_code_10", "1001012345678901", false},
		{"province_code_99", "9901012345678901", true},
		{"all_zeros", "0000000000000000", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNIKFormat(tt.nik)
			if tt.ok && err != nil {
				t.Errorf("expected valid, got %v", err)
			}
			if !tt.ok && err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestTokenizeNIKDeterministic(t *testing.T) {
	key := []byte("test-key-32-bytes-long-for-hmac!")
	nik := "3201012345678901"

	tok1 := TokenizeNIK(nik, key)
	tok2 := TokenizeNIK(nik, key)
	if tok1 != tok2 {
		t.Error("tokenization should be deterministic")
	}
	if len(tok1) != 64 {
		t.Errorf("token length: got %d, want 64", len(tok1))
	}
}

func TestTokenizeNIKDifferentKeys(t *testing.T) {
	nik := "3201012345678901"
	tok1 := TokenizeNIK(nik, []byte("key1"))
	tok2 := TokenizeNIK(nik, []byte("key2"))
	if tok1 == tok2 {
		t.Error("different keys should produce different tokens")
	}
}

func TestTokenizeNIKDifferentNIKs(t *testing.T) {
	key := []byte("same-key")
	tok1 := TokenizeNIK("3201012345678901", key)
	tok2 := TokenizeNIK("3201012345678902", key)
	if tok1 == tok2 {
		t.Error("different NIKs should produce different tokens")
	}
}

func TestTokenizeNIKHexFormat(t *testing.T) {
	tok := TokenizeNIK("3201012345678901", []byte("key"))
	for _, c := range tok {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("non-hex char: %c", c)
			break
		}
	}
}

func TestMaskNIKEdgeCases(t *testing.T) {
	tests := []struct {
		name string
		nik  string
		want string
	}{
		{"standard", "3201012345678901", "************8901"},
		{"short_4", "1234", "1234"},
		{"short_3", "123", "123"},
		{"empty", "", ""},
		{"single", "1", "1"},
		{"five", "12345", "*2345"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskNIK(tt.nik)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMaskNIKLength(t *testing.T) {
	nik := "3201012345678901"
	masked := MaskNIK(nik)
	if len(masked) != len(nik) {
		t.Errorf("masked length differs: %d vs %d", len(masked), len(nik))
	}
}

func TestMaskNIKLastFourPreserved(t *testing.T) {
	nik := "3201012345678901"
	masked := MaskNIK(nik)
	last4 := masked[len(masked)-4:]
	if last4 != "8901" {
		t.Errorf("last 4: got %q, want %q", last4, "8901")
	}
}

func TestMaskNIKStarsCount(t *testing.T) {
	nik := "3201012345678901"
	masked := MaskNIK(nik)
	stars := strings.Count(masked, "*")
	if stars != 12 {
		t.Errorf("star count: got %d, want 12", stars)
	}
}

func TestTokenizeNIKEmptyKey(t *testing.T) {
	// Empty key should still work (HMAC accepts it)
	tok := TokenizeNIK("3201012345678901", []byte(""))
	if len(tok) != 64 {
		t.Errorf("token length: got %d", len(tok))
	}
}

func TestValidateNIKRegexInvariance(t *testing.T) {
	// Verify the regex variable is properly initialized
	if nikRegex == nil {
		t.Fatal("nikRegex should be initialized")
	}
	if !nikRegex.MatchString("1234567890123456") {
		t.Error("regex should match 16 digits")
	}
	if nikRegex.MatchString("123456789012345") {
		t.Error("regex should not match 15 digits")
	}
}
