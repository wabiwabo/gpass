package cbor

import (
	"errors"
	"testing"
)

// TestReadHead_TruncatedAtEachWidth covers the readHead unexpected-end
// branches for the 1/2/4/8-byte length encodings (info=24/25/26/27).
// Each is its own subtest so a regression points at the exact width.
func TestReadHead_TruncatedAtEachWidth(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		// MajorUint=0, info=24 → 1-byte follow, but follow byte is missing.
		{"info=24 truncated", []byte{0x18}},
		// info=25 → 2-byte follow, only 1 byte present.
		{"info=25 truncated", []byte{0x19, 0x00}},
		// info=26 → 4-byte follow, only 3 bytes present.
		{"info=26 truncated", []byte{0x1a, 0x00, 0x00, 0x00}},
		// info=27 → 8-byte follow, only 7 bytes present.
		{"info=27 truncated", []byte{0x1b, 0, 0, 0, 0, 0, 0, 0}},
		// Empty buffer hits the very first guard.
		{"empty", []byte{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := NewDecoder(tc.data)
			_, _, err := d.readHead()
			if !errors.Is(err, ErrUnexpectedEnd) {
				t.Errorf("err = %v, want ErrUnexpectedEnd", err)
			}
		})
	}
}

// TestDecodeInt_TypeMismatchAndNegative covers the negative-int branch
// of DecodeInt and the type-mismatch branch (string fed to int).
func TestDecodeInt_TypeMismatchAndNegative(t *testing.T) {
	// Encode -10 → CBOR head 0x29 (major 1, value 9 → -1-9 = -10)
	d := NewDecoder([]byte{0x29})
	got, err := d.DecodeInt()
	if err != nil {
		t.Fatalf("DecodeInt(-10): %v", err)
	}
	if got != -10 {
		t.Errorf("got %d, want -10", got)
	}

	// Type mismatch: bytes header (major=2, len=0) → DecodeInt rejects.
	d2 := NewDecoder([]byte{0x40})
	if _, err := d2.DecodeInt(); !errors.Is(err, ErrInvalidType) {
		t.Errorf("type mismatch err = %v, want ErrInvalidType", err)
	}
}

// TestDecodeBytes_TypeMismatchAndTruncated covers the wrong-major and
// length-overflow branches in DecodeBytes/DecodeText.
func TestDecodeBytes_TypeMismatchAndTruncated(t *testing.T) {
	// Wrong major: an unsigned int (0x01 = uint 1) decoded as bytes.
	d := NewDecoder([]byte{0x01})
	if _, err := d.DecodeBytes(); !errors.Is(err, ErrInvalidType) {
		t.Errorf("wrong major: err = %v", err)
	}

	// Bytes header claims length 5 but only 2 follow-bytes are present.
	d2 := NewDecoder([]byte{0x45, 0xaa, 0xbb})
	if _, err := d2.DecodeBytes(); !errors.Is(err, ErrUnexpectedEnd) {
		t.Errorf("truncated bytes: err = %v", err)
	}

	// Same shape for DecodeText.
	d3 := NewDecoder([]byte{0x65, 'h', 'i'}) // claims 5, has 2
	if _, err := d3.DecodeText(); !errors.Is(err, ErrUnexpectedEnd) {
		t.Errorf("truncated text: err = %v", err)
	}

	// And the type-mismatch branch for text.
	d4 := NewDecoder([]byte{0x01})
	if _, err := d4.DecodeText(); !errors.Is(err, ErrInvalidType) {
		t.Errorf("text wrong major: err = %v", err)
	}
}

// TestDecodeArrayMapHead_TypeMismatch covers the wrong-major branches
// of DecodeArrayHead and DecodeMapHead.
func TestDecodeArrayMapHead_TypeMismatch(t *testing.T) {
	d := NewDecoder([]byte{0x01}) // uint, not array
	if _, err := d.DecodeArrayHead(); !errors.Is(err, ErrInvalidType) {
		t.Errorf("array on uint: %v", err)
	}
	d2 := NewDecoder([]byte{0x01}) // uint, not map
	if _, err := d2.DecodeMapHead(); !errors.Is(err, ErrInvalidType) {
		t.Errorf("map on uint: %v", err)
	}
}

// TestDecodeBool_AllBranches covers true/false/null/empty.
func TestDecodeBool_AllBranches(t *testing.T) {
	d := NewDecoder([]byte{SimpleTrue})
	if v, err := d.DecodeBool(); err != nil || !v {
		t.Errorf("true: v=%v err=%v", v, err)
	}
	d = NewDecoder([]byte{SimpleFalse})
	if v, err := d.DecodeBool(); err != nil || v {
		t.Errorf("false: v=%v err=%v", v, err)
	}
	// null is neither true nor false → ErrInvalidType
	d = NewDecoder([]byte{SimpleNull})
	if _, err := d.DecodeBool(); !errors.Is(err, ErrInvalidType) {
		t.Errorf("null: err = %v", err)
	}
	// empty buffer → ErrUnexpectedEnd
	d = NewDecoder([]byte{})
	if _, err := d.DecodeBool(); !errors.Is(err, ErrUnexpectedEnd) {
		t.Errorf("empty: err = %v", err)
	}
}

// TestDecodeUint_NegativeAndTypeMismatch covers DecodeUint's wrong-major
// branch (negative int decoded as uint must error).
func TestDecodeUint_TypeMismatch(t *testing.T) {
	d := NewDecoder([]byte{0x29}) // -10
	if _, err := d.DecodeUint(); !errors.Is(err, ErrInvalidType) {
		t.Errorf("negint on uint: err = %v", err)
	}
}

// TestRemaining covers the trivial Remaining helper which had no test.
func TestRemaining(t *testing.T) {
	d := NewDecoder([]byte{0x01, 0x02, 0x03})
	if d.Remaining() != 3 {
		t.Errorf("Remaining = %d, want 3", d.Remaining())
	}
	_, _ = d.DecodeUint()
	if d.Remaining() != 2 {
		t.Errorf("after one byte: Remaining = %d, want 2", d.Remaining())
	}
}
