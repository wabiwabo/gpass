package validate

import (
	"fmt"
	"strings"
	"testing"
)

func TestNIKFormat_LeadingZerosPreserved(t *testing.T) {
	// Province code 11 (Aceh) with leading 1, still valid
	nik := "1101010101010001"
	if err := NIKFormat(nik); err != nil {
		t.Errorf("NIKFormat(%q) should accept province 11: %v", nik, err)
	}
}

func TestNIKFormat_AllValidProvinceCodes(t *testing.T) {
	for code := 11; code <= 94; code++ {
		nik := fmt.Sprintf("%02d01010101010001", code)
		t.Run(fmt.Sprintf("province_%02d", code), func(t *testing.T) {
			if err := NIKFormat(nik); err != nil {
				t.Errorf("NIKFormat(%q) should accept province code %d: %v", nik, code, err)
			}
		})
	}
}

func TestNIKFormat_AllInvalidProvinceCodes(t *testing.T) {
	for code := 0; code <= 10; code++ {
		nik := fmt.Sprintf("%02d01010101010001", code)
		t.Run(fmt.Sprintf("province_%02d", code), func(t *testing.T) {
			if err := NIKFormat(nik); err == nil {
				t.Errorf("NIKFormat(%q) should reject province code %d", nik, code)
			}
		})
	}
}

func TestIsEmail_PlusAlias(t *testing.T) {
	if err := IsEmail("email", "user+tag@example.com"); err != nil {
		t.Errorf("IsEmail should accept plus alias: %v", err)
	}
}

func TestIsEmail_ConsecutiveDotsInLocalPart(t *testing.T) {
	// The regex allows consecutive dots in the local part.
	// This documents current behavior (defense in depth is handled elsewhere).
	err := IsEmail("email", "user..name@example.com")
	if err != nil {
		t.Errorf("current regex accepts consecutive dots in local part: %v", err)
	}
}

func TestIsEmail_MissingDomainExtension(t *testing.T) {
	if err := IsEmail("email", "user@localhost"); err == nil {
		t.Error("IsEmail should reject email without domain extension")
	}
}

func TestIsEmail_EmptyLocalPart(t *testing.T) {
	if err := IsEmail("email", "@example.com"); err == nil {
		t.Error("IsEmail should reject empty local part")
	}
}

func TestIsURL_WithPort(t *testing.T) {
	if err := IsURL("url", "https://example.com:8080/path"); err != nil {
		t.Errorf("IsURL should accept URL with port: %v", err)
	}
}

func TestIsURL_WithQueryString(t *testing.T) {
	if err := IsURL("url", "https://example.com/path?key=value&other=1"); err != nil {
		t.Errorf("IsURL should accept URL with query string: %v", err)
	}
}

func TestIsUUID_Uppercase(t *testing.T) {
	if err := IsUUID("id", "550E8400-E29B-41D4-A716-446655440000"); err != nil {
		t.Errorf("IsUUID should accept uppercase UUID: %v", err)
	}
}

func TestIsUUID_MixedCase(t *testing.T) {
	if err := IsUUID("id", "550e8400-E29B-41d4-A716-446655440000"); err != nil {
		t.Errorf("IsUUID should accept mixed case UUID: %v", err)
	}
}

func TestMinLength_Unicode(t *testing.T) {
	// Emoji is 4 bytes in UTF-8 but len() counts bytes, not runes.
	// MinLength uses len() which is byte-based.
	emoji := "😀😀😀" // 3 emojis = 12 bytes
	if err := MinLength("field", emoji, 12); err != nil {
		t.Errorf("MinLength should pass for emoji string with byte count: %v", err)
	}
	if err := MinLength("field", emoji, 13); err == nil {
		t.Error("MinLength should fail when byte length is less than min")
	}
}

func TestMaxLength_Unicode(t *testing.T) {
	// MaxLength also uses len() (byte-based)
	emoji := "😀😀" // 2 emojis = 8 bytes
	if err := MaxLength("field", emoji, 8); err != nil {
		t.Errorf("MaxLength should pass for emoji string at exact byte count: %v", err)
	}
	if err := MaxLength("field", emoji, 7); err == nil {
		t.Error("MaxLength should fail when byte length exceeds max")
	}
}

func TestRequired_WhitespaceOnly(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"spaces", "   "},
		{"tabs", "\t\t"},
		{"newlines", "\n\n"},
		{"mixed whitespace", " \t\n\r "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Required("field", tt.value); err == nil {
				t.Errorf("Required should reject whitespace-only string %q", tt.value)
			}
		})
	}
}

func TestErrors_AccumulationAndFormat(t *testing.T) {
	var errs Errors

	errs.Add(Required("name", ""))
	errs.Add(Required("email", ""))
	errs.Add(Required("phone", ""))

	if !errs.HasErrors() {
		t.Fatal("expected errors")
	}

	if len(errs.All()) != 3 {
		t.Fatalf("expected 3 errors, got %d", len(errs.All()))
	}

	msg := errs.Error()
	// Errors should be joined by "; "
	if !strings.Contains(msg, "; ") {
		t.Errorf("error message should contain '; ' separator: %q", msg)
	}
	if !strings.Contains(msg, "name is required") {
		t.Errorf("error message should contain 'name is required': %q", msg)
	}
	if !strings.Contains(msg, "email is required") {
		t.Errorf("error message should contain 'email is required': %q", msg)
	}
	if !strings.Contains(msg, "phone is required") {
		t.Errorf("error message should contain 'phone is required': %q", msg)
	}
}

func TestErrors_AddNilDoesNothing(t *testing.T) {
	var errs Errors
	for i := 0; i < 10; i++ {
		errs.Add(nil)
	}
	if errs.HasErrors() {
		t.Error("adding nil errors should not create errors")
	}
	if len(errs.All()) != 0 {
		t.Errorf("expected 0 errors, got %d", len(errs.All()))
	}
}

func TestErrors_SingleError(t *testing.T) {
	var errs Errors
	errs.Add(fmt.Errorf("single error"))
	msg := errs.Error()
	if msg != "single error" {
		t.Errorf("single error message should have no separator, got %q", msg)
	}
}

func TestIsEmail_VariousFormats(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"with subdomain", "user@mail.example.com", false},
		{"with percent", "user%tag@example.com", false},
		{"with hyphen in domain", "user@my-domain.com", false},
		{"missing TLD", "user@example", true},
		{"double at", "user@@example.com", true},
		{"trailing dot in domain", "user@example.com.", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := IsEmail("email", tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsEmail(%q) error = %v, wantErr %v", tt.email, err, tt.wantErr)
			}
		})
	}
}

func TestNIKFormat_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		nik     string
		wantErr bool
	}{
		{"min valid NIK", "1101010101010001", false},
		{"all nines", "9999999999999999", true}, // Province 99 > 94.
		{"with spaces", "3201 0101 0101 0001", true},
		{"empty", "", true},
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
