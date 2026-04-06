package reqvalidator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidate_AllValid(t *testing.T) {
	data := map[string]string{
		"name":  "John Doe",
		"email": "john@example.com",
		"nik":   "3201120509870001",
	}
	rules := []Rule{
		{Field: "name", Required: true, MinLen: 2, MaxLen: 100},
		{Field: "email", Required: true, Pattern: "email"},
		{Field: "nik", Required: true, Pattern: "nik"},
	}

	result := Validate(data, rules)
	if !result.Valid {
		t.Errorf("should be valid: %v", result.Errors)
	}
}

func TestValidate_Required_Missing(t *testing.T) {
	data := map[string]string{}
	rules := []Rule{
		{Field: "name", Required: true},
	}

	result := Validate(data, rules)
	if result.Valid {
		t.Error("missing required field should fail")
	}
	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(result.Errors))
	}
	if result.Errors[0].Code != "required" {
		t.Errorf("error code: got %q", result.Errors[0].Code)
	}
}

func TestValidate_Required_Empty(t *testing.T) {
	data := map[string]string{"name": "   "}
	rules := []Rule{
		{Field: "name", Required: true},
	}

	result := Validate(data, rules)
	if result.Valid {
		t.Error("whitespace-only should fail required")
	}
}

func TestValidate_MinLength(t *testing.T) {
	data := map[string]string{"name": "AB"}
	rules := []Rule{
		{Field: "name", MinLen: 3},
	}

	result := Validate(data, rules)
	if result.Valid {
		t.Error("too short should fail")
	}
	if result.Errors[0].Code != "min_length" {
		t.Errorf("code: got %q", result.Errors[0].Code)
	}
}

func TestValidate_MaxLength(t *testing.T) {
	data := map[string]string{"name": "ABCDEFGHIJK"}
	rules := []Rule{
		{Field: "name", MaxLen: 5},
	}

	result := Validate(data, rules)
	if result.Valid {
		t.Error("too long should fail")
	}
}

func TestValidate_EmailPattern(t *testing.T) {
	tests := []struct {
		email string
		valid bool
	}{
		{"user@example.com", true},
		{"a@b.id", true},
		{"invalid", false},
		{"@no-user.com", true}, // simple validator only checks @ and .
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			result := Validate(
				map[string]string{"email": tt.email},
				[]Rule{{Field: "email", Pattern: "email"}},
			)
			if result.Valid != tt.valid {
				t.Errorf("email %q: valid=%v, want %v", tt.email, result.Valid, tt.valid)
			}
		})
	}
}

func TestValidate_NIKPattern(t *testing.T) {
	tests := []struct {
		nik   string
		valid bool
	}{
		{"3201120509870001", true},
		{"12345", false},          // too short
		{"320112050987000A", false}, // contains letter
		{"32011205098700012", false}, // too long
	}

	for _, tt := range tests {
		t.Run(tt.nik, func(t *testing.T) {
			result := Validate(
				map[string]string{"nik": tt.nik},
				[]Rule{{Field: "nik", Pattern: "nik"}},
			)
			if result.Valid != tt.valid {
				t.Errorf("nik %q: valid=%v, want %v", tt.nik, result.Valid, tt.valid)
			}
		})
	}
}

func TestValidate_PhoneIDPattern(t *testing.T) {
	tests := []struct {
		phone string
		valid bool
	}{
		{"+6281234567890", true},
		{"+6221234567", true},
		{"081234567890", false},
		{"+1234567890", false},
	}

	for _, tt := range tests {
		t.Run(tt.phone, func(t *testing.T) {
			result := Validate(
				map[string]string{"phone": tt.phone},
				[]Rule{{Field: "phone", Pattern: "phone_id"}},
			)
			if result.Valid != tt.valid {
				t.Errorf("phone %q: valid=%v, want %v", tt.phone, result.Valid, tt.valid)
			}
		})
	}
}

func TestValidate_UUIDPattern(t *testing.T) {
	result := Validate(
		map[string]string{"id": "550e8400-e29b-41d4-a716-446655440000"},
		[]Rule{{Field: "id", Pattern: "uuid"}},
	)
	if !result.Valid {
		t.Error("valid UUID should pass")
	}

	result = Validate(
		map[string]string{"id": "not-a-uuid"},
		[]Rule{{Field: "id", Pattern: "uuid"}},
	)
	if result.Valid {
		t.Error("invalid UUID should fail")
	}
}

func TestValidate_CustomRule(t *testing.T) {
	result := Validate(
		map[string]string{"age": "15"},
		[]Rule{{
			Field: "age",
			Custom: func(value string) error {
				if value < "17" {
					return fmt.Errorf("must be at least 17 years old")
				}
				return nil
			},
		}},
	)
	if result.Valid {
		t.Error("custom rule should fail for age 15")
	}
}

func TestValidate_OptionalField_Empty(t *testing.T) {
	data := map[string]string{"name": "John"}
	rules := []Rule{
		{Field: "name", Required: true},
		{Field: "nickname", MinLen: 3}, // optional, not in data
	}

	result := Validate(data, rules)
	if !result.Valid {
		t.Error("optional empty field should not fail")
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	data := map[string]string{}
	rules := []Rule{
		{Field: "name", Required: true},
		{Field: "email", Required: true},
		{Field: "phone", Required: true},
	}

	result := Validate(data, rules)
	if result.Valid {
		t.Error("should fail")
	}
	if len(result.Errors) != 3 {
		t.Errorf("expected 3 errors, got %d", len(result.Errors))
	}
}

func TestDecodeAndValidate(t *testing.T) {
	body := `{"name":"Alice","email":"alice@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))

	rules := []Rule{
		{Field: "name", Required: true},
		{Field: "email", Required: true, Pattern: "email"},
	}

	data, result, err := DecodeAndValidate(req, 1024, rules)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid {
		t.Errorf("should be valid: %v", result.Errors)
	}
	if data["name"] != "Alice" {
		t.Errorf("name: got %q", data["name"])
	}
}

func TestDecodeAndValidate_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("not json"))
	_, _, err := DecodeAndValidate(req, 1024, nil)
	if err == nil {
		t.Error("should fail on invalid JSON")
	}
}

func TestDecodeAndValidate_NilBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Body = nil
	_, _, err := DecodeAndValidate(req, 1024, nil)
	if err == nil {
		t.Error("should fail on nil body")
	}
}

func TestWriteValidationError(t *testing.T) {
	result := ValidationResult{
		Valid: false,
		Errors: []FieldError{
			{Field: "email", Code: "required", Message: "email is required"},
		},
	}

	w := httptest.NewRecorder()
	WriteValidationError(w, result)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] != "validation_failed" {
		t.Errorf("error: got %v", resp["error"])
	}
}
