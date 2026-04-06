package durationfmt

import (
	"testing"
	"time"
)

func TestFormat(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{30 * time.Second, "30s"},
		{5 * time.Minute, "5m"},
		{2*time.Hour + 30*time.Minute, "2h30m"},
		{24 * time.Hour, "1d"},
		{3*24*time.Hour + 12*time.Hour + 30*time.Minute, "3d12h30m"},
		{1 * time.Second, "1s"},
		{61 * time.Second, "1m1s"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := Format(tt.d)
			if got != tt.want {
				t.Errorf("Format(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

func TestFormat_Negative(t *testing.T) {
	got := Format(-5 * time.Minute)
	if got != "-5m" {
		t.Errorf("Format(-5m) = %q", got)
	}
}

func TestFormatShort(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{48 * time.Hour, "2d"},
		{3 * time.Hour, "3h"},
		{45 * time.Minute, "45m"},
		{15 * time.Second, "15s"},
		{500 * time.Millisecond, "500ms"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatShort(tt.d)
			if got != tt.want {
				t.Errorf("FormatShort(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"30s", 30 * time.Second},
		{"5m", 5 * time.Minute},
		{"2h", 2 * time.Hour},
		{"1d", 24 * time.Hour},
		{"2h30m", 2*time.Hour + 30*time.Minute},
		{"1d12h30m15s", 24*time.Hour + 12*time.Hour + 30*time.Minute + 15*time.Second},
		{"500ms", 500 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("Parse(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParse_Negative(t *testing.T) {
	got, err := Parse("-5m")
	if err != nil {
		t.Fatal(err)
	}
	if got != -5*time.Minute {
		t.Errorf("got %v, want -5m", got)
	}
}

func TestParse_Errors(t *testing.T) {
	tests := []string{
		"",
		"abc",
		"5x",
		"m5",
	}
	for _, input := range tests {
		_, err := Parse(input)
		if err == nil {
			t.Errorf("Parse(%q) should error", input)
		}
	}
}

func TestRoundTrip(t *testing.T) {
	original := 3*24*time.Hour + 5*time.Hour + 30*time.Minute + 15*time.Second
	formatted := Format(original)
	parsed, err := Parse(formatted)
	if err != nil {
		t.Fatalf("Parse(%q): %v", formatted, err)
	}
	if parsed != original {
		t.Errorf("roundtrip: %v → %q → %v", original, formatted, parsed)
	}
}
