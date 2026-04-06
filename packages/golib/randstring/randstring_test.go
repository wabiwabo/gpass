package randstring

import (
	"strings"
	"testing"
)

func TestGenerate_Length(t *testing.T) {
	tests := []struct {
		length int
		chars  CharSet
	}{
		{10, Alphanumeric},
		{32, Hex},
		{6, Numeric},
		{64, URLSafe},
		{1, Alpha},
	}
	for _, tt := range tests {
		s, err := Generate(tt.length, tt.chars)
		if err != nil {
			t.Fatalf("Generate(%d): %v", tt.length, err)
		}
		if len(s) != tt.length {
			t.Errorf("len = %d, want %d", len(s), tt.length)
		}
	}
}

func TestGenerate_CharsetRespected(t *testing.T) {
	s, _ := Generate(1000, Numeric)
	for _, c := range s {
		if c < '0' || c > '9' {
			t.Errorf("non-numeric char: %c", c)
		}
	}
}

func TestGenerate_HexOnly(t *testing.T) {
	s, _ := Generate(1000, Hex)
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("non-hex char: %c", c)
		}
	}
}

func TestGenerate_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		s, _ := Generate(32, Alphanumeric)
		if seen[s] {
			t.Fatal("duplicate string")
		}
		seen[s] = true
	}
}

func TestGenerate_ZeroLength(t *testing.T) {
	s, err := Generate(0, Alphanumeric)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if s != "" {
		t.Errorf("expected empty string, got %q", s)
	}
}

func TestGenerate_EmptyCharset(t *testing.T) {
	_, err := Generate(10, "")
	if err == nil {
		t.Error("expected error for empty charset")
	}
}

func TestAlphanumericString(t *testing.T) {
	s, err := AlphanumericString(16)
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 16 {
		t.Errorf("len = %d", len(s))
	}
}

func TestHexString(t *testing.T) {
	s, err := HexString(32)
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 32 {
		t.Errorf("len = %d", len(s))
	}
	for _, c := range s {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("invalid hex char: %c", c)
		}
	}
}

func TestNumericString(t *testing.T) {
	s, err := NumericString(6)
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 6 {
		t.Errorf("len = %d", len(s))
	}
}

func TestURLSafeString(t *testing.T) {
	s, err := URLSafeString(48)
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 48 {
		t.Errorf("len = %d", len(s))
	}
	for _, c := range s {
		if !strings.ContainsRune(string(URLSafe), c) {
			t.Errorf("non-URLSafe char: %c", c)
		}
	}
}
