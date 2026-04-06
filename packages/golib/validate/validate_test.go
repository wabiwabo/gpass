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
		{"valid province 99", "9901010101010001", false},
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
