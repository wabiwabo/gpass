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
		if !strings.Contains(value, "@") || !strings.Contains(value, ".") {
			return fmt.Errorf("%s must be a valid email address", field)
		}
	case "uuid":
		parts := strings.Split(value, "-")
		if len(parts) != 5 {
			return fmt.Errorf("%s must be a valid UUID", field)
		}
	case "nik":
		if len(value) != 16 {
			return fmt.Errorf("%s must be exactly 16 digits", field)
		}
		for _, c := range value {
			if c < '0' || c > '9' {
				return fmt.Errorf("%s must contain only digits", field)
			}
		}
	case "phone_id":
		if !strings.HasPrefix(value, "+62") {
			return fmt.Errorf("%s must start with +62 (Indonesian format)", field)
		}
	}
	return nil
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
