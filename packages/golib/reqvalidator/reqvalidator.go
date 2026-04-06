package reqvalidator

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// FieldError represents a validation error for a specific field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// ValidationResult holds the outcome of request validation.
type ValidationResult struct {
	Valid  bool         `json:"valid"`
	Errors []FieldError `json:"errors,omitempty"`
}

// AddError adds a field validation error.
func (v *ValidationResult) AddError(field, code, message string) {
	v.Valid = false
	v.Errors = append(v.Errors, FieldError{
		Field:   field,
		Message: message,
		Code:    code,
	})
}

// Rule defines a validation rule for a field.
type Rule struct {
	Field    string
	Required bool
	MinLen   int
	MaxLen   int
	Pattern  string // "email", "uuid", "nik", "phone_id"
	Custom   func(value string) error
}

// Validate validates a map of field values against rules.
func Validate(data map[string]string, rules []Rule) ValidationResult {
	result := ValidationResult{Valid: true}

	for _, rule := range rules {
		value := data[rule.Field]

		if rule.Required && strings.TrimSpace(value) == "" {
			result.AddError(rule.Field, "required", rule.Field+" is required")
			continue
		}

		if value == "" {
			continue // optional and empty, skip other checks
		}

		if rule.MinLen > 0 && len(value) < rule.MinLen {
			result.AddError(rule.Field, "min_length",
				fmt.Sprintf("%s must be at least %d characters", rule.Field, rule.MinLen))
		}

		if rule.MaxLen > 0 && len(value) > rule.MaxLen {
			result.AddError(rule.Field, "max_length",
				fmt.Sprintf("%s must not exceed %d characters", rule.Field, rule.MaxLen))
		}

		if rule.Pattern != "" {
			if err := validatePattern(rule.Field, value, rule.Pattern); err != nil {
				result.AddError(rule.Field, "invalid_format", err.Error())
			}
		}

		if rule.Custom != nil {
			if err := rule.Custom(value); err != nil {
				result.AddError(rule.Field, "custom", err.Error())
			}
		}
	}

	return result
}

func validatePattern(field, value, pattern string) error {
	switch pattern {
	case "email":
		return validateEmail(field, value)
	case "uuid":
		return validateUUID(field, value)
	case "nik":
		return validateNIK(field, value)
	case "phone_id":
		return validatePhoneID(field, value)
	case "npwp":
		return validateNPWP(field, value)
	case "date":
		return validateDate(field, value)
	case "url":
		return validateURL(field, value)
	case "ip":
		return validateIP(field, value)
	case "alpha":
		return validateAlpha(field, value)
	case "alphanumeric":
		return validateAlphanumeric(field, value)
	case "numeric":
		return validateNumeric(field, value)
	}
	return nil
}

func validateEmail(field, value string) error {
	at := strings.IndexByte(value, '@')
	if at < 1 { // Must have at least 1 char before @.
		return fmt.Errorf("%s must be a valid email address", field)
	}
	domain := value[at+1:]
	if len(domain) < 3 { // At least "a.b".
		return fmt.Errorf("%s must be a valid email address", field)
	}
	if !strings.Contains(domain, ".") {
		return fmt.Errorf("%s must be a valid email address", field)
	}
	// Check local part doesn't contain dangerous chars.
	local := value[:at]
	if len(local) > 64 {
		return fmt.Errorf("%s local part must not exceed 64 characters", field)
	}
	if len(domain) > 253 {
		return fmt.Errorf("%s domain must not exceed 253 characters", field)
	}
	// Reject obvious injection patterns.
	for _, c := range local {
		if c == '<' || c == '>' || c == '(' || c == ')' || c == ';' || c == '\\' || c == '"' || c == '\'' {
			return fmt.Errorf("%s contains invalid characters", field)
		}
	}
	// Domain must not start/end with dot or hyphen.
	if domain[0] == '.' || domain[0] == '-' || domain[len(domain)-1] == '.' || domain[len(domain)-1] == '-' {
		return fmt.Errorf("%s has invalid domain format", field)
	}
	return nil
}

func validateUUID(field, value string) error {
	// UUID format: 8-4-4-4-12 hex chars = 36 total.
	if len(value) != 36 {
		return fmt.Errorf("%s must be a valid UUID (36 characters)", field)
	}
	parts := strings.Split(value, "-")
	if len(parts) != 5 {
		return fmt.Errorf("%s must be a valid UUID", field)
	}
	expectedLens := []int{8, 4, 4, 4, 12}
	for i, part := range parts {
		if len(part) != expectedLens[i] {
			return fmt.Errorf("%s has invalid UUID segment length", field)
		}
		for _, c := range part {
			if !isHexChar(c) {
				return fmt.Errorf("%s must contain only hexadecimal characters", field)
			}
		}
	}
	return nil
}

func validateNIK(field, value string) error {
	if len(value) != 16 {
		return fmt.Errorf("%s must be exactly 16 digits", field)
	}
	for _, c := range value {
		if c < '0' || c > '9' {
			return fmt.Errorf("%s must contain only digits", field)
		}
	}
	// Validate province code (first 2 digits): 11-94 range.
	provinceCode := (value[0]-'0')*10 + (value[1] - '0')
	if provinceCode < 11 || provinceCode > 94 {
		return fmt.Errorf("%s has invalid province code", field)
	}
	return nil
}

func validatePhoneID(field, value string) error {
	if !strings.HasPrefix(value, "+62") {
		return fmt.Errorf("%s must start with +62 (Indonesian format)", field)
	}
	digits := value[3:]
	if len(digits) < 8 || len(digits) > 13 {
		return fmt.Errorf("%s must have 8-13 digits after +62", field)
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return fmt.Errorf("%s must contain only digits after country code", field)
		}
	}
	return nil
}

// validateNPWP validates Indonesian tax ID (Nomor Pokok Wajib Pajak).
// Format: XX.XXX.XXX.X-XXX.XXX (15 digits with separators) or 15 raw digits.
func validateNPWP(field, value string) error {
	// Strip separators.
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		if r == '.' || r == '-' {
			return -1 // Strip.
		}
		return r
	}, value)

	if len(digits) != 15 {
		return fmt.Errorf("%s must be a valid NPWP (15 digits)", field)
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return fmt.Errorf("%s must contain only digits", field)
		}
	}
	return nil
}

func validateDate(field, value string) error {
	// Accept ISO 8601 date format: YYYY-MM-DD.
	if len(value) != 10 {
		return fmt.Errorf("%s must be in YYYY-MM-DD format", field)
	}
	if value[4] != '-' || value[7] != '-' {
		return fmt.Errorf("%s must be in YYYY-MM-DD format", field)
	}
	for _, i := range []int{0, 1, 2, 3, 5, 6, 8, 9} {
		if value[i] < '0' || value[i] > '9' {
			return fmt.Errorf("%s must contain valid date digits", field)
		}
	}
	// Basic month/day range check.
	month := (value[5]-'0')*10 + (value[6] - '0')
	day := (value[8]-'0')*10 + (value[9] - '0')
	if month < 1 || month > 12 {
		return fmt.Errorf("%s has invalid month", field)
	}
	if day < 1 || day > 31 {
		return fmt.Errorf("%s has invalid day", field)
	}
	return nil
}

func validateURL(field, value string) error {
	if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
		return fmt.Errorf("%s must start with http:// or https://", field)
	}
	// Must have host after scheme.
	scheme := "http://"
	if strings.HasPrefix(value, "https://") {
		scheme = "https://"
	}
	host := value[len(scheme):]
	if len(host) == 0 {
		return fmt.Errorf("%s must have a valid host", field)
	}
	// Reject obvious injection.
	for _, c := range host {
		if c == '<' || c == '>' || c == '"' || c == '\'' || c == ' ' || c == '{' || c == '}' {
			return fmt.Errorf("%s contains invalid characters", field)
		}
	}
	return nil
}

func validateIP(field, value string) error {
	// Simple IPv4 validation.
	parts := strings.Split(value, ".")
	if len(parts) != 4 {
		return fmt.Errorf("%s must be a valid IPv4 address", field)
	}
	for _, part := range parts {
		if len(part) == 0 || len(part) > 3 {
			return fmt.Errorf("%s has invalid IP octet", field)
		}
		n := 0
		for _, c := range part {
			if c < '0' || c > '9' {
				return fmt.Errorf("%s must contain only digits in IP octets", field)
			}
			n = n*10 + int(c-'0')
		}
		if n > 255 {
			return fmt.Errorf("%s has IP octet out of range", field)
		}
	}
	return nil
}

func validateAlpha(field, value string) error {
	for _, c := range value {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
			return fmt.Errorf("%s must contain only alphabetic characters", field)
		}
	}
	return nil
}

func validateAlphanumeric(field, value string) error {
	for _, c := range value {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			return fmt.Errorf("%s must contain only alphanumeric characters", field)
		}
	}
	return nil
}

func validateNumeric(field, value string) error {
	for _, c := range value {
		if c < '0' || c > '9' {
			return fmt.Errorf("%s must contain only digits", field)
		}
	}
	return nil
}

func isHexChar(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// DecodeAndValidate reads a JSON body, decodes it into a map, and validates against rules.
func DecodeAndValidate(r *http.Request, maxBodySize int64, rules []Rule) (map[string]string, ValidationResult, error) {
	if r.Body == nil {
		return nil, ValidationResult{}, fmt.Errorf("request body is empty")
	}

	limited := io.LimitReader(r.Body, maxBodySize)
	var raw map[string]interface{}
	if err := json.NewDecoder(limited).Decode(&raw); err != nil {
		return nil, ValidationResult{}, fmt.Errorf("invalid JSON: %w", err)
	}

	data := make(map[string]string, len(raw))
	for k, v := range raw {
		data[k] = fmt.Sprintf("%v", v)
	}

	result := Validate(data, rules)
	return data, result, nil
}

// WriteValidationError writes a 400 response with validation errors.
func WriteValidationError(w http.ResponseWriter, result ValidationResult) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   "validation_failed",
		"message": "Request validation failed",
		"details": result.Errors,
	})
}
