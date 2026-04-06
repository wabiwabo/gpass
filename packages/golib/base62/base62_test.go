package base62

import (
	"testing"
)

func TestEncodeInt(t *testing.T) {
	tests := []struct {
		n    uint64
		want string
	}{
		{0, "0"},
		{1, "1"},
		{9, "9"},
		{10, "A"},
		{35, "Z"},
		{36, "a"},
		{61, "z"},
		{62, "10"},
		{100, "1c"},
	}
	for _, tt := range tests {
		got := EncodeInt(tt.n)
		if got != tt.want {
			t.Errorf("EncodeInt(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestDecodeInt(t *testing.T) {
	tests := []struct {
		s    string
		want uint64
	}{
		{"0", 0},
		{"1", 1},
		{"A", 10},
		{"z", 61},
		{"10", 62},
		{"1c", 100},
	}
	for _, tt := range tests {
		got, err := DecodeInt(tt.s)
		if err != nil {
			t.Fatalf("DecodeInt(%q): %v", tt.s, err)
		}
		if got != tt.want {
			t.Errorf("DecodeInt(%q) = %d, want %d", tt.s, got, tt.want)
		}
	}
}

func TestRoundTrip_Int(t *testing.T) {
	for _, n := range []uint64{0, 1, 42, 100, 999, 12345, 1000000, 18446744073709551615} {
		encoded := EncodeInt(n)
		decoded, err := DecodeInt(encoded)
		if err != nil {
			t.Fatalf("roundtrip %d: %v", n, err)
		}
		if decoded != n {
			t.Errorf("roundtrip %d → %q → %d", n, encoded, decoded)
		}
	}
}

func TestEncode_Bytes(t *testing.T) {
	data := []byte{1, 2, 3}
	encoded := Encode(data)
	if encoded == "" {
		t.Error("should not be empty")
	}
	if !IsValid(encoded) {
		t.Errorf("encoded %q contains invalid chars", encoded)
	}
}

func TestDecode_Bytes(t *testing.T) {
	data := []byte{0x48, 0x65, 0x6c, 0x6c, 0x6f} // "Hello"
	encoded := Encode(data)
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if string(decoded) != "Hello" {
		t.Errorf("Decode = %q, want Hello", decoded)
	}
}

func TestEncode_Empty(t *testing.T) {
	if Encode(nil) != "" {
		t.Error("nil should encode to empty")
	}
	if Encode([]byte{}) != "" {
		t.Error("empty should encode to empty")
	}
}

func TestDecode_Empty(t *testing.T) {
	data, err := Decode("")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if data != nil {
		t.Errorf("data = %v, want nil", data)
	}
}

func TestDecode_InvalidChar(t *testing.T) {
	_, err := Decode("abc!def")
	if err == nil {
		t.Error("should error on invalid char")
	}
}

func TestDecodeInt_Empty(t *testing.T) {
	_, err := DecodeInt("")
	if err == nil {
		t.Error("should error on empty")
	}
}

func TestDecodeInt_InvalidChar(t *testing.T) {
	_, err := DecodeInt("abc!")
	if err == nil {
		t.Error("should error on invalid char")
	}
}

func TestIsValid(t *testing.T) {
	if !IsValid("abc123XYZ") {
		t.Error("should be valid")
	}
	if !IsValid("") {
		t.Error("empty should be valid")
	}
	if IsValid("abc!") {
		t.Error("! should be invalid")
	}
	if IsValid("abc def") {
		t.Error("space should be invalid")
	}
}

func TestEncodeInt_Compact(t *testing.T) {
	// Base62 should be more compact than decimal for large numbers
	encoded := EncodeInt(1000000)
	if len(encoded) >= 7 { // "1000000" is 7 chars
		t.Errorf("base62 should be shorter: %q (len %d)", encoded, len(encoded))
	}
}
