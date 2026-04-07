package bytefmt

import (
	"strings"
	"testing"
)

// TestFormat_NegativeAndZeroAndSubKB exercises the negate / zero / raw
// "B" branches of formatUnits, plus the integer-vs-fractional paths in
// formatFloat. The existing tests cover the happy path KB/MB/GB only.
func TestFormat_NegativeAndZeroAndSubKB(t *testing.T) {
	cases := []struct {
		name string
		in   int64
		want string
	}{
		{"zero", 0, "0 B"},
		{"sub-KB", 999, "999 B"},
		{"exactly 1 KB integer", 1000, "1 KB"},
		{"1.5 KB one-decimal", 1500, "1.5 KB"},
		{"1.25 MB two-decimal", 1_250_000, "1.25 MB"},
		{"negative KB", -2048, "-2.05 KB"}, // 2048/1000 = 2.048 → 2-decimal float
		{"large negative", -1_500_000, "-1.5 MB"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Format(tc.in)
			if got != tc.want {
				t.Errorf("Format(%d) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestFormatIEC_Branches mirrors the SI test against IEC powers-of-2.
func TestFormatIEC_Branches(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1 KiB"},
		{1024 * 1024, "1 MiB"},
		{1024*1024 + 512*1024, "1.5 MiB"},
		{1024 * 1024 * 1024, "1 GiB"},
	}
	for _, tc := range cases {
		got := FormatIEC(tc.in)
		if got != tc.want {
			t.Errorf("FormatIEC(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestParse_AllUnits covers every unit-suffix branch in Parse, including
// the implicit-bytes case (no suffix), the case-insensitivity (lowercase
// "kb"), and the IEC variants.
func TestParse_AllUnits(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"0", 0},
		{"100", 100},      // implicit bytes
		{"100 B", 100},    // explicit bytes
		{"1KB", 1000},     // no space
		{"1 kb", 1000},    // case-insensitive
		{"1.5MB", 1_500_000},
		{"2 GB", 2_000_000_000},
		{"1 KiB", 1024},
		{"1 MiB", 1024 * 1024},
		{"1 GiB", 1024 * 1024 * 1024},
		{"1 TB", 1_000_000_000_000},
		{"1 PB", 1_000_000_000_000_000},
		{"1 TiB", 1024 * 1024 * 1024 * 1024},
		{"1 PiB", 1024 * 1024 * 1024 * 1024 * 1024},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := Parse(tc.in)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("Parse(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

// TestParse_Errors covers each error branch.
func TestParse_Errors_Cov(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "empty string"},
		{"   ", "empty string"},
		{"abc", "invalid number"},
		{"1.5 frobs", "unknown unit"},
		{"10 ZB", "unknown unit"}, // zettabytes not supported
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			_, err := Parse(tc.in)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %q, want substring %q", err.Error(), tc.want)
			}
		})
	}
}

// TestFormat_ParseRoundTrip pins that Format → Parse round-trips for
// values that don't lose precision in the decimal representation.
func TestFormat_ParseRoundTrip(t *testing.T) {
	values := []int64{
		0, 1, 999, 1000, 1500, 1_000_000, 2_500_000_000,
	}
	for _, v := range values {
		s := Format(v)
		got, err := Parse(s)
		if err != nil {
			t.Errorf("Parse(%q) from %d: %v", s, v, err)
			continue
		}
		if got != v {
			t.Errorf("round-trip lost precision: %d → %q → %d", v, s, got)
		}
	}
}
