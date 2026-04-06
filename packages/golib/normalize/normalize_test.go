package normalize

import (
	"testing"
)

func TestName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"  JOHN  DOE  ", "John Doe"},
		{"muhammad bin abdullah", "Muhammad bin Abdullah"},
		{"siti binti hassan", "Siti binti Hassan"},
		{"van den berg", "van Den Berg"}, // Only "van" is lowercase; "den" is title-cased.
		{"", ""},
		{"  ", ""},
		{"BUDI SANTOSO", "Budi Santoso"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Name(tt.input)
			if got != tt.want {
				t.Errorf("Name(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestPhone(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"+6281234567890", "+6281234567890"},
		{"081234567890", "+6281234567890"},
		{"6281234567890", "+6281234567890"},
		{"+62 812 3456 7890", "+6281234567890"},
		{"0812-3456-7890", "+6281234567890"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Phone(tt.input)
			if got != tt.want {
				t.Errorf("Phone(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestEmail(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"User@Example.COM", "user@example.com"},
		{"  user@test.id  ", "user@test.id"},
		{"user+tag@gmail.com", "user@gmail.com"},
		{"", ""},
		{"noemail", "noemail"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Email(tt.input)
			if got != tt.want {
				t.Errorf("Email(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNIK(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"3201120509870001", "3201120509870001"},
		{"32.01.12.050987.0001", "3201120509870001"},
		{"12345", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NIK(tt.input)
			if got != tt.want {
				t.Errorf("NIK(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNPWP(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"01.234.567.8-901.234", "012345678901234"},
		{"012345678901234", "012345678901234"},
		{"12345", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NPWP(tt.input)
			if got != tt.want {
				t.Errorf("NPWP(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestAddress(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"jl. sudirman no. 1", "Jalan sudirman No. 1"},     // Expands abbreviations, preserves existing case.
		{"gg. melati rt. 01 rw. 02", "Gang melati RT 01 RW 02"}, // Same.
		{"", ""},
		{"  multiple   spaces  ", "multiple spaces"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Address(tt.input)
			if got != tt.want {
				t.Errorf("Address(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCompanyName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"pt garudapass indonesia", "PT Garudapass Indonesia"},
		{"cv maju jaya", "CV Maju Jaya"},
		{"pt. bank central asia tbk.", "PT Bank Central Asia Tbk"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := CompanyName(tt.input)
			if got != tt.want {
				t.Errorf("CompanyName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
