// Package redactjson provides PII-aware JSON redaction.
// Walks JSON structures and redacts sensitive fields by name
// before logging or external transmission, per UU PDP compliance.
package redactjson

import (
	"encoding/json"
	"strings"
)

// Default sensitive field names that should be redacted.
var DefaultSensitiveFields = map[string]bool{
	"nik":          true,
	"password":     true,
	"secret":       true,
	"token":        true,
	"access_token": true,
	"refresh_token": true,
	"id_token":     true,
	"api_key":      true,
	"private_key":  true,
	"ssn":          true,
	"credit_card":  true,
	"cvv":          true,
	"pin":          true,
}

// Redactor walks JSON and redacts sensitive fields.
type Redactor struct {
	SensitiveFields map[string]bool
	Replacement     string
}

// New creates a redactor with default sensitive fields and "[REDACTED]" replacement.
func New() *Redactor {
	fields := make(map[string]bool, len(DefaultSensitiveFields))
	for k, v := range DefaultSensitiveFields {
		fields[k] = v
	}
	return &Redactor{
		SensitiveFields: fields,
		Replacement:     "[REDACTED]",
	}
}

// AddField marks a field name as sensitive.
func (r *Redactor) AddField(name string) {
	r.SensitiveFields[strings.ToLower(name)] = true
}

// Redact walks v and replaces sensitive field values with the replacement string.
func (r *Redactor) Redact(v interface{}) interface{} {
	switch x := v.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(x))
		for k, val := range x {
			if r.SensitiveFields[strings.ToLower(k)] {
				result[k] = r.Replacement
			} else {
				result[k] = r.Redact(val)
			}
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(x))
		for i, item := range x {
			result[i] = r.Redact(item)
		}
		return result
	default:
		return v
	}
}

// RedactBytes parses JSON, redacts sensitive fields, and re-encodes.
func (r *Redactor) RedactBytes(data []byte) ([]byte, error) {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	redacted := r.Redact(v)
	return json.Marshal(redacted)
}

// RedactString is a string-based convenience wrapper.
func (r *Redactor) RedactString(s string) (string, error) {
	out, err := r.RedactBytes([]byte(s))
	if err != nil {
		return "", err
	}
	return string(out), nil
}
