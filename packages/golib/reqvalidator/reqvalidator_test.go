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
		{"first.last@company.co.id", true},
		{"invalid", false},
		{"@no-user.com", false},           // No local part.
		{"user@", false},                   // No domain.
		{"user@.com", false},               // Domain starts with dot.
		{"user@com", false},                // No dot in domain.
		{"user@example.com-", false},       // Domain ends with hyphen.
		{"<script>@evil.com", false},       // Injection in local part.
		{"user\"inject@evil.com", false},   // Quote injection.
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			result := Validate(
				map[string]string{"email": tt.email},
				[]Rule{{Field: "email", Pattern: "email"}},
			)
			if result.Valid != tt.valid {
				t.Errorf("email %q: valid=%v, want %v, errors=%v", tt.email, result.Valid, tt.valid, result.Errors)
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
		{"+622123456789", true},
		{"+62812345678", true},
		{"081234567890", false},  // Missing +62.
		{"+1234567890", false},   // Wrong country code.
		{"+621234567", false},    // Too short (7 digits).
		{"+6281abc", false},      // Non-digit after code.
		{"+6281234567890123", false}, // Too long (14 digits).
	}

	for _, tt := range tests {
		t.Run(tt.phone, func(t *testing.T) {
			result := Validate(
				map[string]string{"phone": tt.phone},
				[]Rule{{Field: "phone", Pattern: "phone_id"}},
			)
			if result.Valid != tt.valid {
				t.Errorf("phone %q: valid=%v, want %v, errors=%v", tt.phone, result.Valid, tt.valid, result.Errors)
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

func TestValidate_UUIDStrict(t *testing.T) {
	tests := []struct {
		uuid  string
		valid bool
	}{
		{"550e8400-e29b-41d4-a716-446655440000", true},
		{"123e4567-e89b-12d3-a456-426614174000", true},
		{"not-a-uuid", false},
		{"550e8400-e29b-41d4-a716", false},                 // Too short.
		{"550e8400-e29b-41d4-a716-44665544000g", false},     // Non-hex.
		{"550e8400e29b41d4a716446655440000", false},          // No dashes.
		{"550e8400-e29b-41d4-a716-4466554400000", false},    // Too long.
	}

	for _, tt := range tests {
		t.Run(tt.uuid, func(t *testing.T) {
			result := Validate(
				map[string]string{"id": tt.uuid},
				[]Rule{{Field: "id", Pattern: "uuid"}},
			)
			if result.Valid != tt.valid {
				t.Errorf("uuid %q: valid=%v, want %v", tt.uuid, result.Valid, tt.valid)
			}
		})
	}
}

func TestValidate_NIKStrict(t *testing.T) {
	tests := []struct {
		nik   string
		valid bool
	}{
		{"3201120509870001", true},  // Province 32 (Jawa Barat).
		{"1101010101010001", true},  // Province 11 (Aceh).
		{"9401010101010001", true},  // Province 94 (Papua).
		{"0501010101010001", false}, // Province 05 — invalid (< 11).
		{"9901010101010001", false}, // Province 99 — invalid (> 94).
		{"12345", false},            // Too short.
		{"320112050987000A", false}, // Contains letter.
	}

	for _, tt := range tests {
		t.Run(tt.nik, func(t *testing.T) {
			result := Validate(
				map[string]string{"nik": tt.nik},
				[]Rule{{Field: "nik", Pattern: "nik"}},
			)
			if result.Valid != tt.valid {
				t.Errorf("nik %q: valid=%v, want %v, errors=%v", tt.nik, result.Valid, tt.valid, result.Errors)
			}
		})
	}
}

func TestValidate_NPWPPattern(t *testing.T) {
	tests := []struct {
		npwp  string
		valid bool
	}{
		{"01.234.567.8-901.234", true},    // With separators.
		{"012345678901234", true},          // Raw digits.
		{"01.234.567.8-901.23", false},     // Only 14 digits.
		{"01.234.567.8-901.2345", false},   // 16 digits.
		{"0123456789A1234", false},          // Non-digit.
	}

	for _, tt := range tests {
		t.Run(tt.npwp, func(t *testing.T) {
			result := Validate(
				map[string]string{"npwp": tt.npwp},
				[]Rule{{Field: "npwp", Pattern: "npwp"}},
			)
			if result.Valid != tt.valid {
				t.Errorf("npwp %q: valid=%v, want %v, errors=%v", tt.npwp, result.Valid, tt.valid, result.Errors)
			}
		})
	}
}

func TestValidate_DatePattern(t *testing.T) {
	tests := []struct {
		date  string
		valid bool
	}{
		{"2024-01-15", true},
		{"1999-12-31", true},
		{"2024-13-01", false},  // Invalid month.
		{"2024-01-32", false},  // Invalid day.
		{"2024/01/15", false},  // Wrong separator.
		{"24-01-15", false},    // Too short.
		{"2024-1-15", false},   // Single digit month.
	}

	for _, tt := range tests {
		t.Run(tt.date, func(t *testing.T) {
			result := Validate(
				map[string]string{"date": tt.date},
				[]Rule{{Field: "date", Pattern: "date"}},
			)
			if result.Valid != tt.valid {
				t.Errorf("date %q: valid=%v, want %v", tt.date, result.Valid, tt.valid)
			}
		})
	}
}

func TestValidate_URLPattern(t *testing.T) {
	tests := []struct {
		url   string
		valid bool
	}{
		{"https://example.com", true},
		{"http://localhost:8080/path", true},
		{"https://api.garudapass.id/v1", true},
		{"ftp://example.com", false},       // Wrong scheme.
		{"example.com", false},             // No scheme.
		{"https://", false},                // No host.
		{"https://<script>", false},         // Injection.
		{"https://evil.com/path with spaces", false}, // Space in URL.
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			result := Validate(
				map[string]string{"url": tt.url},
				[]Rule{{Field: "url", Pattern: "url"}},
			)
			if result.Valid != tt.valid {
				t.Errorf("url %q: valid=%v, want %v", tt.url, result.Valid, tt.valid)
			}
		})
	}
}

func TestValidate_IPPattern(t *testing.T) {
	tests := []struct {
		ip    string
		valid bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"0.0.0.0", true},
		{"255.255.255.255", true},
		{"256.1.1.1", false},     // Octet > 255.
		{"1.2.3", false},         // Only 3 octets.
		{"1.2.3.4.5", false},     // 5 octets.
		{"abc.def.ghi.jkl", false}, // Non-numeric.
		{"1.2.3.", false},         // Empty octet.
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			result := Validate(
				map[string]string{"ip": tt.ip},
				[]Rule{{Field: "ip", Pattern: "ip"}},
			)
			if result.Valid != tt.valid {
				t.Errorf("ip %q: valid=%v, want %v, errors=%v", tt.ip, result.Valid, tt.valid, result.Errors)
			}
		})
	}
}

func TestValidate_AlphaPattern(t *testing.T) {
	result := Validate(map[string]string{"v": "HelloWorld"}, []Rule{{Field: "v", Pattern: "alpha"}})
	if !result.Valid {
		t.Error("alpha chars should pass")
	}
	result = Validate(map[string]string{"v": "Hello123"}, []Rule{{Field: "v", Pattern: "alpha"}})
	if result.Valid {
		t.Error("digits should fail alpha")
	}
}

func TestValidate_AlphanumericPattern(t *testing.T) {
	result := Validate(map[string]string{"v": "Hello123"}, []Rule{{Field: "v", Pattern: "alphanumeric"}})
	if !result.Valid {
		t.Error("alphanumeric should pass")
	}
	result = Validate(map[string]string{"v": "Hello-123"}, []Rule{{Field: "v", Pattern: "alphanumeric"}})
	if result.Valid {
		t.Error("hyphen should fail alphanumeric")
	}
}

func TestValidate_NumericPattern(t *testing.T) {
	result := Validate(map[string]string{"v": "12345"}, []Rule{{Field: "v", Pattern: "numeric"}})
	if !result.Valid {
		t.Error("digits should pass numeric")
	}
	result = Validate(map[string]string{"v": "123a5"}, []Rule{{Field: "v", Pattern: "numeric"}})
	if result.Valid {
		t.Error("letter should fail numeric")
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
