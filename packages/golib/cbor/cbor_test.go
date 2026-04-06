package cbor

import (
	"testing"
)

func TestUintRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		val  uint64
	}{
		{"zero", 0},
		{"small", 23},
		{"one_byte", 24},
		{"max_uint8", 255},
		{"uint16", 256},
		{"max_uint16", 65535},
		{"uint32", 65536},
		{"max_uint32", 4294967295},
		{"uint64", 4294967296},
		{"large", 1<<62 - 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc := NewEncoder(16)
			enc.EncodeUint(tt.val)
			dec := NewDecoder(enc.Bytes())
			got, err := dec.DecodeUint()
			if err != nil {
				t.Fatalf("decode error: %v", err)
			}
			if got != tt.val {
				t.Errorf("got %d, want %d", got, tt.val)
			}
			if dec.Remaining() != 0 {
				t.Errorf("remaining bytes: %d", dec.Remaining())
			}
		})
	}
}

func TestIntRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		val  int64
	}{
		{"zero", 0},
		{"positive", 42},
		{"negative_one", -1},
		{"negative", -100},
		{"negative_large", -1000000},
		{"max_positive", 1<<62 - 1},
		{"min_negative", -1 << 62},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc := NewEncoder(16)
			enc.EncodeInt(tt.val)
			dec := NewDecoder(enc.Bytes())
			got, err := dec.DecodeInt()
			if err != nil {
				t.Fatalf("decode error: %v", err)
			}
			if got != tt.val {
				t.Errorf("got %d, want %d", got, tt.val)
			}
		})
	}
}

func TestBytesRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"small", []byte{1, 2, 3}},
		{"medium", make([]byte, 256)},
		{"binary", []byte{0x00, 0xFF, 0xDE, 0xAD, 0xBE, 0xEF}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc := NewEncoder(len(tt.data) + 16)
			enc.EncodeBytes(tt.data)
			dec := NewDecoder(enc.Bytes())
			got, err := dec.DecodeBytes()
			if err != nil {
				t.Fatalf("decode error: %v", err)
			}
			if len(got) != len(tt.data) {
				t.Fatalf("length: got %d, want %d", len(got), len(tt.data))
			}
			for i := range got {
				if got[i] != tt.data[i] {
					t.Errorf("byte %d: got 0x%02x, want 0x%02x", i, got[i], tt.data[i])
					break
				}
			}
		})
	}
}

func TestTextRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"empty", ""},
		{"ascii", "hello world"},
		{"unicode", "日本語テスト"},
		{"emoji", "Hello 🌍"},
		{"long", string(make([]byte, 300))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc := NewEncoder(len(tt.text) + 16)
			enc.EncodeText(tt.text)
			dec := NewDecoder(enc.Bytes())
			got, err := dec.DecodeText()
			if err != nil {
				t.Fatalf("decode error: %v", err)
			}
			if got != tt.text {
				t.Errorf("got %q, want %q", got, tt.text)
			}
		})
	}
}

func TestBoolRoundTrip(t *testing.T) {
	for _, v := range []bool{true, false} {
		enc := NewEncoder(4)
		enc.EncodeBool(v)
		dec := NewDecoder(enc.Bytes())
		got, err := dec.DecodeBool()
		if err != nil {
			t.Fatalf("decode error: %v", err)
		}
		if got != v {
			t.Errorf("got %v, want %v", got, v)
		}
	}
}

func TestArrayRoundTrip(t *testing.T) {
	enc := NewEncoder(32)
	enc.EncodeArrayHead(3)
	enc.EncodeUint(1)
	enc.EncodeUint(2)
	enc.EncodeUint(3)

	dec := NewDecoder(enc.Bytes())
	n, err := dec.DecodeArrayHead()
	if err != nil {
		t.Fatalf("decode array head: %v", err)
	}
	if n != 3 {
		t.Fatalf("array length: got %d, want 3", n)
	}
	for i := 0; i < n; i++ {
		v, err := dec.DecodeUint()
		if err != nil {
			t.Fatalf("decode item %d: %v", i, err)
		}
		if v != uint64(i+1) {
			t.Errorf("item %d: got %d, want %d", i, v, i+1)
		}
	}
}

func TestMapRoundTrip(t *testing.T) {
	enc := NewEncoder(64)
	enc.EncodeMapHead(2)
	enc.EncodeText("name")
	enc.EncodeText("alice")
	enc.EncodeText("age")
	enc.EncodeUint(30)

	dec := NewDecoder(enc.Bytes())
	n, err := dec.DecodeMapHead()
	if err != nil {
		t.Fatalf("decode map head: %v", err)
	}
	if n != 2 {
		t.Fatalf("map pairs: got %d, want 2", n)
	}

	k1, _ := dec.DecodeText()
	v1, _ := dec.DecodeText()
	k2, _ := dec.DecodeText()
	v2, _ := dec.DecodeUint()

	if k1 != "name" || v1 != "alice" {
		t.Errorf("pair 1: got %q=%q", k1, v1)
	}
	if k2 != "age" || v2 != 30 {
		t.Errorf("pair 2: got %q=%d", k2, v2)
	}
}

func TestNull(t *testing.T) {
	enc := NewEncoder(4)
	enc.EncodeNull()
	if len(enc.Bytes()) != 1 {
		t.Errorf("null should be 1 byte, got %d", len(enc.Bytes()))
	}
	if enc.Bytes()[0] != SimpleNull {
		t.Errorf("got 0x%02x, want 0x%02x", enc.Bytes()[0], SimpleNull)
	}
}

func TestTag(t *testing.T) {
	enc := NewEncoder(16)
	enc.EncodeTag(0) // tag 0 = date/time string
	enc.EncodeText("2024-01-01T00:00:00Z")

	dec := NewDecoder(enc.Bytes())
	major, val, err := dec.readHead()
	if err != nil {
		t.Fatalf("decode tag: %v", err)
	}
	if major != MajorTag {
		t.Errorf("major: got %d, want %d", major, MajorTag)
	}
	if val != 0 {
		t.Errorf("tag: got %d, want 0", val)
	}
	text, err := dec.DecodeText()
	if err != nil {
		t.Fatalf("decode text: %v", err)
	}
	if text != "2024-01-01T00:00:00Z" {
		t.Errorf("text: got %q", text)
	}
}

func TestReset(t *testing.T) {
	enc := NewEncoder(16)
	enc.EncodeUint(42)
	if len(enc.Bytes()) == 0 {
		t.Fatal("should have data")
	}
	enc.Reset()
	if len(enc.Bytes()) != 0 {
		t.Errorf("after reset: got %d bytes", len(enc.Bytes()))
	}
}

func TestDecodeEmptyData(t *testing.T) {
	dec := NewDecoder([]byte{})
	_, err := dec.DecodeUint()
	if err != ErrUnexpectedEnd {
		t.Errorf("expected ErrUnexpectedEnd, got %v", err)
	}
}

func TestDecodeTypeMismatch(t *testing.T) {
	enc := NewEncoder(4)
	enc.EncodeText("hello")

	dec := NewDecoder(enc.Bytes())
	_, err := dec.DecodeUint()
	if err != ErrInvalidType {
		t.Errorf("expected ErrInvalidType, got %v", err)
	}
}

func TestDecodeTruncatedBytes(t *testing.T) {
	enc := NewEncoder(16)
	enc.EncodeBytes([]byte{1, 2, 3, 4, 5})
	// Truncate the data
	data := enc.Bytes()[:3]

	dec := NewDecoder(data)
	_, err := dec.DecodeBytes()
	if err != ErrUnexpectedEnd {
		t.Errorf("expected ErrUnexpectedEnd, got %v", err)
	}
}

func TestComplexStructure(t *testing.T) {
	// Encode: {"items": [1, -2, "three"], "ok": true}
	enc := NewEncoder(64)
	enc.EncodeMapHead(2)
	enc.EncodeText("items")
	enc.EncodeArrayHead(3)
	enc.EncodeUint(1)
	enc.EncodeInt(-2)
	enc.EncodeText("three")
	enc.EncodeText("ok")
	enc.EncodeBool(true)

	dec := NewDecoder(enc.Bytes())
	n, _ := dec.DecodeMapHead()
	if n != 2 {
		t.Fatalf("map pairs: %d", n)
	}

	k1, _ := dec.DecodeText()
	if k1 != "items" {
		t.Errorf("key1: %q", k1)
	}
	arrLen, _ := dec.DecodeArrayHead()
	if arrLen != 3 {
		t.Fatalf("array len: %d", arrLen)
	}
	v1, _ := dec.DecodeUint()
	v2, _ := dec.DecodeInt()
	v3, _ := dec.DecodeText()
	if v1 != 1 || v2 != -2 || v3 != "three" {
		t.Errorf("array: %d, %d, %q", v1, v2, v3)
	}

	k2, _ := dec.DecodeText()
	if k2 != "ok" {
		t.Errorf("key2: %q", k2)
	}
	boolVal, _ := dec.DecodeBool()
	if !boolVal {
		t.Error("expected true")
	}
	if dec.Remaining() != 0 {
		t.Errorf("remaining: %d", dec.Remaining())
	}
}

func TestEncoderCapacityGrows(t *testing.T) {
	enc := NewEncoder(1) // tiny initial capacity
	for i := 0; i < 100; i++ {
		enc.EncodeUint(uint64(i))
	}
	dec := NewDecoder(enc.Bytes())
	for i := 0; i < 100; i++ {
		v, err := dec.DecodeUint()
		if err != nil {
			t.Fatalf("decode %d: %v", i, err)
		}
		if v != uint64(i) {
			t.Errorf("item %d: got %d", i, v)
		}
	}
}

func BenchmarkEncodeUint(b *testing.B) {
	enc := NewEncoder(1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		enc.Reset()
		enc.EncodeUint(uint64(i))
	}
}

func BenchmarkDecodeUint(b *testing.B) {
	enc := NewEncoder(16)
	enc.EncodeUint(123456789)
	data := enc.Bytes()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dec := NewDecoder(data)
		dec.DecodeUint()
	}
}
