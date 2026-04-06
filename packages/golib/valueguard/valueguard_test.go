package valueguard

import (
	"testing"
	"time"
)

func TestRequired(t *testing.T) {
	if Required("name", "John") != nil {
		t.Error("non-empty should pass")
	}
	if Required("name", "") == nil {
		t.Error("empty should fail")
	}
	if Required("name", "   ") == nil {
		t.Error("whitespace should fail")
	}
}

func TestMinLength(t *testing.T) {
	if MinLength("pw", "abcdef", 6) != nil {
		t.Error("exact min should pass")
	}
	if MinLength("pw", "abc", 6) == nil {
		t.Error("below min should fail")
	}
}

func TestMaxLength(t *testing.T) {
	if MaxLength("name", "abc", 10) != nil {
		t.Error("below max should pass")
	}
	if MaxLength("name", "abcdefghijk", 10) == nil {
		t.Error("above max should fail")
	}
}

func TestRange(t *testing.T) {
	if Range("age", 25, 18, 65) != nil {
		t.Error("in range should pass")
	}
	if Range("age", 17, 18, 65) == nil {
		t.Error("below range should fail")
	}
	if Range("age", 66, 18, 65) == nil {
		t.Error("above range should fail")
	}
}

func TestPositive(t *testing.T) {
	if Positive("count", 1) != nil {
		t.Error("positive should pass")
	}
	if Positive("count", 0) == nil {
		t.Error("zero should fail")
	}
	if Positive("count", -1) == nil {
		t.Error("negative should fail")
	}
}

func TestEmail(t *testing.T) {
	valid := []string{"user@example.com", "a.b@domain.co.id", "test+tag@gmail.com"}
	for _, e := range valid {
		if Email("email", e) != nil {
			t.Errorf("%q should be valid", e)
		}
	}

	invalid := []string{"", "notanemail", "@domain.com", "user@", "user@.com"}
	for _, e := range invalid {
		if Email("email", e) == nil {
			t.Errorf("%q should be invalid", e)
		}
	}
}

func TestOneOf(t *testing.T) {
	if OneOf("status", "active", "active", "inactive", "pending") != nil {
		t.Error("valid value should pass")
	}
	if OneOf("status", "deleted", "active", "inactive") == nil {
		t.Error("invalid value should fail")
	}
}

func TestNotFuture(t *testing.T) {
	if NotFuture("dob", time.Now().Add(-1*time.Hour)) != nil {
		t.Error("past time should pass")
	}
	if NotFuture("dob", time.Now().Add(1*time.Hour)) == nil {
		t.Error("future time should fail")
	}
}

func TestNotPast(t *testing.T) {
	if NotPast("expires", time.Now().Add(1*time.Hour)) != nil {
		t.Error("future time should pass")
	}
	if NotPast("expires", time.Now().Add(-1*time.Hour)) == nil {
		t.Error("past time should fail")
	}
}

func TestIP(t *testing.T) {
	if IP("addr", "192.168.1.1") != nil {
		t.Error("valid IPv4 should pass")
	}
	if IP("addr", "::1") != nil {
		t.Error("valid IPv6 should pass")
	}
	if IP("addr", "not-an-ip") == nil {
		t.Error("invalid IP should fail")
	}
}

func TestMatches(t *testing.T) {
	if Matches("code", "ABC-123", `^[A-Z]+-\d+$`) != nil {
		t.Error("matching pattern should pass")
	}
	if Matches("code", "abc", `^[A-Z]+$`) == nil {
		t.Error("non-matching should fail")
	}
}

func TestValidate(t *testing.T) {
	err := Validate(
		Required("name", "John"),
		Email("email", "john@example.com"),
		MinLength("password", "secret123", 8),
	)
	if err != nil {
		t.Errorf("all valid should pass: %v", err)
	}

	err = Validate(
		Required("name", ""),
		Email("email", "john@example.com"),
	)
	if err == nil {
		t.Error("should return first error")
	}
	if err.Field != "name" {
		t.Errorf("Field = %q, want name", err.Field)
	}
}

func TestValidateAll(t *testing.T) {
	errs := ValidateAll(
		Required("name", ""),
		Email("email", "bad"),
		MinLength("pw", "ab", 8),
	)
	if len(errs) != 3 {
		t.Errorf("errs = %d, want 3", len(errs))
	}
}

func TestValidateAll_NoErrors(t *testing.T) {
	errs := ValidateAll(
		Required("name", "John"),
		Email("email", "j@x.com"),
	)
	if len(errs) != 0 {
		t.Errorf("errs = %d, want 0", len(errs))
	}
}

func TestValidationError_Error(t *testing.T) {
	err := Required("name", "")
	if err.Error() != "name: name is required" {
		t.Errorf("Error = %q", err.Error())
	}
}

func TestValidationError_Fields(t *testing.T) {
	err := Required("email", "")
	if err.Field != "email" {
		t.Errorf("Field = %q", err.Field)
	}
	if err.Code != "required" {
		t.Errorf("Code = %q", err.Code)
	}
}
