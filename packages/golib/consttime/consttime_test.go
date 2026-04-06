package consttime

import (
	"testing"
)

func TestEqual(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{"identical", "secret", "secret", true},
		{"different", "secret", "public", false},
		{"empty_both", "", "", true},
		{"empty_one", "", "a", false},
		{"length_differ", "short", "longer", false},
		{"unicode", "日本語", "日本語", true},
		{"unicode_differ", "日本語", "中国語", false},
		{"similar", "password1", "password2", false},
		{"case_sensitive", "ABC", "abc", false},
		{"null_bytes", "a\x00b", "a\x00b", true},
		{"null_bytes_differ", "a\x00b", "a\x00c", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Equal(tt.a, tt.b); got != tt.want {
				t.Errorf("Equal(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestEqualBytes(t *testing.T) {
	tests := []struct {
		name string
		a, b []byte
		want bool
	}{
		{"identical", []byte{1, 2, 3}, []byte{1, 2, 3}, true},
		{"different", []byte{1, 2, 3}, []byte{4, 5, 6}, false},
		{"empty_both", []byte{}, []byte{}, true},
		{"nil_both", nil, nil, true},
		{"nil_empty", nil, []byte{}, true},
		{"length_differ", []byte{1, 2}, []byte{1, 2, 3}, false},
		{"single_byte", []byte{0xff}, []byte{0xff}, true},
		{"single_byte_differ", []byte{0xfe}, []byte{0xff}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EqualBytes(tt.a, tt.b); got != tt.want {
				t.Errorf("EqualBytes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSelect(t *testing.T) {
	tests := []struct {
		name     string
		selector int
		a, b     string
		want     string
	}{
		{"select_a", 1, "first", "second", "first"},
		{"select_b", 0, "first", "second", "second"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Select(tt.selector, tt.a, tt.b); got != tt.want {
				t.Errorf("Select(%d, %q, %q) = %q, want %q", tt.selector, tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestIsZero(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"all_zero", []byte{0, 0, 0, 0}, true},
		{"empty", []byte{}, true},
		{"single_zero", []byte{0}, true},
		{"single_nonzero", []byte{1}, false},
		{"last_nonzero", []byte{0, 0, 0, 1}, false},
		{"first_nonzero", []byte{1, 0, 0, 0}, false},
		{"middle_nonzero", []byte{0, 0, 0xff, 0}, false},
		{"large_zero", make([]byte, 1024), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsZero(tt.data); got != tt.want {
				t.Errorf("IsZero() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsZeroLargeNonZero(t *testing.T) {
	data := make([]byte, 4096)
	data[4095] = 0x01
	if IsZero(data) {
		t.Error("IsZero should be false when last byte is nonzero")
	}
}

func TestLengthEqual(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{"same_length", "abc", "xyz", true},
		{"different_length", "ab", "xyz", false},
		{"empty_both", "", "", true},
		{"empty_one", "", "a", false},
		{"unicode_same", "日本", "中国", true},
		{"unicode_differ", "日本語", "ab", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LengthEqual(tt.a, tt.b); got != tt.want {
				t.Errorf("LengthEqual(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestEqualSymmetry(t *testing.T) {
	a, b := "token-abc-123", "token-abc-456"
	if Equal(a, b) != Equal(b, a) {
		t.Error("Equal should be symmetric")
	}
}

func TestEqualBytesSymmetry(t *testing.T) {
	a := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	b := []byte{0xCA, 0xFE, 0xBA, 0xBE}
	if EqualBytes(a, b) != EqualBytes(b, a) {
		t.Error("EqualBytes should be symmetric")
	}
}

func BenchmarkEqual(b *testing.B) {
	s1 := "a]very-long-secret-token-for-benchmarking-purposes-1234567890"
	s2 := "a]very-long-secret-token-for-benchmarking-purposes-1234567891"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Equal(s1, s2)
	}
}

func BenchmarkEqualBytes(b *testing.B) {
	b1 := make([]byte, 256)
	b2 := make([]byte, 256)
	b2[255] = 1
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EqualBytes(b1, b2)
	}
}
