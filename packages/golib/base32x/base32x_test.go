package base32x

import (
	"bytes"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"hello", []byte("hello")},
		{"binary", []byte{0x00, 0xFF, 0xDE, 0xAD, 0xBE, 0xEF}},
		{"long", bytes.Repeat([]byte("abcdef"), 50)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := Encode(tt.data)
			decoded, err := Decode(encoded)
			if err != nil {
				t.Fatalf("decode error: %v", err)
			}
			if !bytes.Equal(decoded, tt.data) {
				t.Errorf("round trip mismatch")
			}
		})
	}
}

func TestNoPadRoundTrip(t *testing.T) {
	data := []byte("test data for no-padding encoding")
	encoded := EncodeNoPad(data)
	if encoded[len(encoded)-1] == '=' {
		t.Error("should not have padding")
	}
	decoded, err := DecodeNoPad(encoded)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if !bytes.Equal(decoded, data) {
		t.Error("round trip mismatch")
	}
}

func TestHexRoundTrip(t *testing.T) {
	data := []byte("hex encoding test")
	encoded := EncodeHex(data)
	decoded, err := DecodeHex(encoded)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if !bytes.Equal(decoded, data) {
		t.Error("round trip mismatch")
	}
}

func TestLowerRoundTrip(t *testing.T) {
	data := []byte("lowercase test")
	encoded := EncodeLower(data)
	// Verify lowercase
	for _, c := range encoded {
		if c >= 'A' && c <= 'Z' {
			t.Errorf("should be lowercase: %c", c)
		}
	}
	decoded, err := DecodeLower(encoded)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if !bytes.Equal(decoded, data) {
		t.Error("round trip mismatch")
	}
}

func TestDecodeInvalid(t *testing.T) {
	_, err := Decode("!!!invalid!!!")
	if err == nil {
		t.Error("expected error for invalid base32")
	}
}

func TestIsValid(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid_padded", Encode([]byte("test")), true},
		{"valid_nopad", EncodeNoPad([]byte("test")), true},
		{"invalid", "!!!###", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValid(tt.input); got != tt.want {
				t.Errorf("IsValid(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestEncodeConsistency(t *testing.T) {
	data := []byte("consistency check")
	e1 := Encode(data)
	e2 := Encode(data)
	if e1 != e2 {
		t.Error("encoding should be deterministic")
	}
}
