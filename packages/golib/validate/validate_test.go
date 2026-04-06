package validate

import (
	"testing"
)

func TestRequired(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"present", "hello", false},
		{"empty", "", true},
		{"whitespace", "   ", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Required("field", tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Required(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestMinLength(t *testing.T) {
	if err := MinLength("name", "abc", 3); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := MinLength("name", "ab", 3); err == nil {
		t.Error("expected error for too-short value")
	}
}

func TestMaxLength(t *testing.T) {
	if err := MaxLength("name", "abc", 5); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := MaxLength("name", "abcdef", 5); err == nil {
		t.Error("expected error for too-long value")
	}
}

func TestIsEmail(t *testing.T) {
	tests := []struct {
		email   string
		wantErr bool
	}{
		{"user@example.com", false},
		{"user+tag@example.co.id", false},
		{"invalid", true},
		{"@example.com", true},
		{"user@", true},
		{"", true},
	}
	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			err := IsEmail("email", tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsEmail(%q) error = %v, wantErr %v", tt.email, err, tt.wantErr)
			}
		})
	}
}

func TestIsURL(t *testing.T) {
	if err := IsURL("url", "https://example.com/path"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := IsURL("url", "not a url"); err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestIsUUID(t *testing.T) {
	tests := []struct {
		uuid    string
		wantErr bool
	}{
		{"550e8400-e29b-41d4-a716-446655440000", false},
		{"not-a-uuid", true},
		{"550e8400-e29b-31d4-a716-446655440000", true}, // version 3, not 4
		{"", true},
	}
	for _, tt := range tests {
		t.Run(tt.uuid, func(t *testing.T) {
			err := IsUUID("id", tt.uuid)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsUUID(%q) error = %v, wantErr %v", tt.uuid, err, tt.wantErr)
			}
		})
	}
}

func TestNIKFormat(t *testing.T) {
	tests := []struct {
		name    string
		nik     string
		wantErr bool
	}{
		{"valid", "3201010101010001", false},
		{"valid province 94", "9401010101010001", false},
		{"invalid province 99", "9901010101010001", true},
		{"too short", "123456", true},
		{"too long", "12345678901234567", true},
		{"letters", "abcdefghijklmnop", true},
		{"invalid province 10", "1001010101010001", true},
		{"invalid province 00", "0001010101010001", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NIKFormat(tt.nik)
			if (err != nil) != tt.wantErr {
				t.Errorf("NIKFormat(%q) error = %v, wantErr %v", tt.nik, err, tt.wantErr)
			}
		})
	}
}

func TestErrors_Accumulation(t *testing.T) {
	var errs Errors

	if errs.HasErrors() {
		t.Error("expected no errors initially")
	}

	errs.Add(nil) // should be ignored
	if errs.HasErrors() {
		t.Error("nil error should not be added")
	}

	errs.Add(Required("name", ""))
	errs.Add(Required("email", ""))

	if !errs.HasErrors() {
		t.Error("expected errors after adding")
	}
	if len(errs.All()) != 2 {
		t.Errorf("expected 2 errors, got %d", len(errs.All()))
	}

	msg := errs.Error()
	if msg == "" {
		t.Error("expected non-empty error message")
	}
}

func TestErrors_EmptyError(t *testing.T) {
	var errs Errors
	if msg := errs.Error(); msg != "" {
		t.Errorf("expected empty error message, got %q", msg)
	}
}

func TestNIKFormat_DateEncoding(t *testing.T) {
	tests := []struct {
		nik   string
		valid bool
		desc  string
	}{
		{"3201120509870001", true, "valid male NIK"},
		{"3201124509870001", true, "valid female NIK (day+40=45)"},
		{"3201120013870001", false, "invalid month 13"},
		{"3201120000870001", false, "invalid day 0"},
		{"3201120032870001", false, "day 32 invalid for male"},
		{"3201120072870001", false, "day 72 invalid for female"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			err := NIKFormat(tt.nik)
			if (err == nil) != tt.valid {
				t.Errorf("NIKFormat(%q): err=%v, want valid=%v", tt.nik, err, tt.valid)
			}
		})
	}
}

func TestNIKFormat_DistrictValidation(t *testing.T) {
	err := NIKFormat("3200120509870001") // District 00 invalid.
	if err == nil {
		t.Error("district 00 should be invalid")
	}
}

func TestNPWPFormat(t *testing.T) {
	tests := []struct {
		npwp  string
		valid bool
	}{
		{"012345678901234", true},
		{"01.234.567.8-901.234", true},
		{"12345", false},
		{"01234567890123A", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.npwp, func(t *testing.T) {
			err := NPWPFormat(tt.npwp)
			if (err == nil) != tt.valid {
				t.Errorf("NPWPFormat(%q): err=%v, want valid=%v", tt.npwp, err, tt.valid)
			}
		})
	}
}

func TestPhoneIDFormat(t *testing.T) {
	tests := []struct {
		phone string
		valid bool
	}{
		{"+6281234567890", true},
		{"+622123456789", true},
		{"081234567890", false},
		{"+621234567", false},      // Too short.
		{"+62812345678901234", false}, // Too long.
		{"+6281abc", false},         // Non-digit.
	}

	for _, tt := range tests {
		t.Run(tt.phone, func(t *testing.T) {
			err := PhoneIDFormat(tt.phone)
			if (err == nil) != tt.valid {
				t.Errorf("PhoneIDFormat(%q): err=%v, want valid=%v", tt.phone, err, tt.valid)
			}
		})
	}
}

func TestNIBFormat(t *testing.T) {
	tests := []struct {
		nib   string
		valid bool
	}{
		{"1234567890123", true},
		{"12345", false},
		{"123456789012A", false},
	}

	for _, tt := range tests {
		t.Run(tt.nib, func(t *testing.T) {
			err := NIBFormat(tt.nib)
			if (err == nil) != tt.valid {
				t.Errorf("NIBFormat(%q): err=%v, want valid=%v", tt.nib, err, tt.valid)
			}
		})
	}
}

func TestAKTAFormat(t *testing.T) {
	tests := []struct {
		akta  string
		valid bool
	}{
		{"12345", true},
		{"123/2024", true},
		{"1-2024", true},
		{"", false},
		{"ABC", false}, // Letters not allowed.
	}

	for _, tt := range tests {
		t.Run(tt.akta, func(t *testing.T) {
			err := AKTAFormat(tt.akta)
			if (err == nil) != tt.valid {
				t.Errorf("AKTAFormat(%q): err=%v, want valid=%v", tt.akta, err, tt.valid)
			}
		})
	}
}

func TestSKFormat(t *testing.T) {
	if err := SKFormat("AHU-12345.AH.01.01"); err != nil {
		t.Errorf("valid SK: %v", err)
	}
	if err := SKFormat("INVALID-123"); err == nil {
		t.Error("invalid SK should fail")
	}
	if err := SKFormat(""); err == nil {
		t.Error("empty SK should fail")
	}
}

func TestIsAlpha(t *testing.T) {
	if err := IsAlpha("name", "Hello"); err != nil {
		t.Error("alpha should pass")
	}
	if err := IsAlpha("name", "Hello123"); err == nil {
		t.Error("digits should fail alpha")
	}
}

func TestIsAlphanumeric(t *testing.T) {
	if err := IsAlphanumeric("code", "ABC123"); err != nil {
		t.Error("alphanumeric should pass")
	}
	if err := IsAlphanumeric("code", "ABC-123"); err == nil {
		t.Error("hyphen should fail alphanumeric")
	}
}

func TestIsNumeric(t *testing.T) {
	if err := IsNumeric("id", "12345"); err != nil {
		t.Error("numeric should pass")
	}
	if err := IsNumeric("id", "123a5"); err == nil {
		t.Error("letter should fail numeric")
	}
}

func TestInRange(t *testing.T) {
	if err := InRange("age", 25, 17, 65); err != nil {
		t.Error("in range should pass")
	}
	if err := InRange("age", 16, 17, 65); err == nil {
		t.Error("below range should fail")
	}
	if err := InRange("age", 66, 17, 65); err == nil {
		t.Error("above range should fail")
	}
}

func TestOneOf(t *testing.T) {
	if err := OneOf("status", "active", []string{"active", "inactive", "deleted"}); err != nil {
		t.Error("valid option should pass")
	}
	if err := OneOf("status", "unknown", []string{"active", "inactive"}); err == nil {
		t.Error("invalid option should fail")
	}
}
