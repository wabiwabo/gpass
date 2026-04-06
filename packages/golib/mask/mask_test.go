package mask

import (
	"testing"
)

func TestNIK(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"3201120509870001", "************0001"},
		{"1234567890123456", "************3456"},
		{"short", "*****"},
	}
	for _, tt := range tests {
		got := NIK(tt.input)
		if got != tt.expected {
			t.Errorf("NIK(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestEmail(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"john@example.com", "j***@example.com"},
		{"a@b.com", "a@b.com"},
		{"alice.bob@company.id", "a********@company.id"},
		{"invalid", "*******"},
	}
	for _, tt := range tests {
		got := Email(tt.input)
		if got != tt.expected {
			t.Errorf("Email(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestPhone(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"+6281234567890", "+62*******7890"},
		{"+628123456", "+62***3456"},
		{"081234567890", "********7890"},
		{"+62", "***"},
	}
	for _, tt := range tests {
		got := Phone(tt.input)
		if got != tt.expected {
			t.Errorf("Phone(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestName(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"John Doe", "J******e"},
		{"Alice", "A***e"},
		{"AB", "**"},
		{"A", "*"},
	}
	for _, tt := range tests {
		got := Name(tt.input)
		if got != tt.expected {
			t.Errorf("Name(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestCreditCard(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"4111111111111111", "************1111"},
		{"4111 1111 1111 1111", "************1111"},
		{"4111-1111-1111-1111", "************1111"},
		{"123", "***"},
	}
	for _, tt := range tests {
		got := CreditCard(tt.input)
		if got != tt.expected {
			t.Errorf("CreditCard(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestNPWP(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"01.234.567.8-901.000", "*****************000"},
		{"AB", "**"},
	}
	for _, tt := range tests {
		got := NPWP(tt.input)
		if got != tt.expected {
			t.Errorf("NPWP(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestPartial(t *testing.T) {
	tests := []struct {
		input    string
		show     int
		expected string
	}{
		{"secret_data", 3, "sec********"},
		{"ab", 5, "ab"},
		{"hello", 0, "*****"},
	}
	for _, tt := range tests {
		got := Partial(tt.input, tt.show)
		if got != tt.expected {
			t.Errorf("Partial(%q, %d) = %q, want %q", tt.input, tt.show, got, tt.expected)
		}
	}
}

func TestFull(t *testing.T) {
	if Full("secret") != "******" {
		t.Errorf("Full: got %q", Full("secret"))
	}
	if Full("") != "" {
		t.Error("empty string should return empty")
	}
}

func TestMap(t *testing.T) {
	data := map[string]string{
		"name":  "John Doe",
		"email": "john@example.com",
		"nik":   "3201120509870001",
		"city":  "Jakarta",
	}

	masked := Map(data, map[string]func(string) string{
		"name":  Name,
		"email": Email,
		"nik":   NIK,
	})

	if masked["name"] != "J******e" {
		t.Errorf("name: got %q", masked["name"])
	}
	if masked["email"] != "j***@example.com" {
		t.Errorf("email: got %q", masked["email"])
	}
	if masked["nik"] != "************0001" {
		t.Errorf("nik: got %q", masked["nik"])
	}
	if masked["city"] != "Jakarta" {
		t.Errorf("city should not be masked: got %q", masked["city"])
	}
}

func TestMap_EmptyFieldMap(t *testing.T) {
	data := map[string]string{"x": "y"}
	masked := Map(data, nil)
	if masked["x"] != "y" {
		t.Error("no sensitive fields means no masking")
	}
}
