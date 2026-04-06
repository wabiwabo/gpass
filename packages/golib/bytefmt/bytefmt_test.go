package bytefmt

import (
	"testing"
)

func TestFormat(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1000, "1 KB"},
		{1500, "1.5 KB"},
		{1000000, "1 MB"},
		{1500000, "1.5 MB"},
		{1000000000, "1 GB"},
		{1000000000000, "1 TB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := Format(tt.bytes)
			if got != tt.want {
				t.Errorf("Format(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestFormatIEC(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{1024, "1 KiB"},
		{1536, "1.5 KiB"},
		{1048576, "1 MiB"},
		{1073741824, "1 GiB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatIEC(tt.bytes)
			if got != tt.want {
				t.Errorf("FormatIEC(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"0 B", 0},
		{"500 B", 500},
		{"1 KB", 1000},
		{"1.5 KB", 1500},
		{"1 MB", 1000000},
		{"1 GB", 1000000000},
		{"1 KiB", 1024},
		{"1 MiB", 1048576},
		{"1 GiB", 1073741824},
		{"100", 100},
		{"2.5GB", 2500000000},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("Parse(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestParse_CaseInsensitive(t *testing.T) {
	got, err := Parse("1 gb")
	if err != nil {
		t.Fatal(err)
	}
	if got != 1000000000 {
		t.Errorf("got %d", got)
	}
}

func TestParse_Errors(t *testing.T) {
	tests := []string{
		"",
		"abc",
		"1 XB",
	}
	for _, input := range tests {
		_, err := Parse(input)
		if err == nil {
			t.Errorf("Parse(%q) should error", input)
		}
	}
}

func TestConstants(t *testing.T) {
	if KB != 1000 {
		t.Errorf("KB = %d", KB)
	}
	if MB != 1000000 {
		t.Errorf("MB = %d", MB)
	}
	if KiB != 1024 {
		t.Errorf("KiB = %d", KiB)
	}
	if MiB != 1048576 {
		t.Errorf("MiB = %d", MiB)
	}
}
