// Package redactlog provides PII-safe log value redaction.
// Masks sensitive fields like NIK, email, phone, and tokens
// before they appear in log output, per UU PDP compliance.
package redactlog

import (
	"strings"
)

// FieldType classifies a field for redaction.
type FieldType int

const (
	FieldGeneral  FieldType = iota
	FieldNIK                // Indonesian national ID
	FieldEmail
	FieldPhone
	FieldToken
	FieldName
	FieldAddress
	FieldPassword
)

// sensitiveKeys maps common key names to field types.
var sensitiveKeys = map[string]FieldType{
	"nik":          FieldNIK,
	"email":        FieldEmail,
	"phone":        FieldPhone,
	"phone_number": FieldPhone,
	"mobile":       FieldPhone,
	"token":        FieldToken,
	"access_token": FieldToken,
	"id_token":     FieldToken,
	"api_key":      FieldToken,
	"password":     FieldPassword,
	"secret":       FieldPassword,
	"name":         FieldName,
	"full_name":    FieldName,
	"address":      FieldAddress,
}

// Classify returns the field type for a given key name.
func Classify(key string) FieldType {
	k := strings.ToLower(key)
	if ft, ok := sensitiveKeys[k]; ok {
		return ft
	}
	return FieldGeneral
}

// Redact masks a value based on its field type.
func Redact(fieldType FieldType, value string) string {
	if value == "" {
		return ""
	}
	switch fieldType {
	case FieldNIK:
		return redactNIK(value)
	case FieldEmail:
		return redactEmail(value)
	case FieldPhone:
		return redactPhone(value)
	case FieldToken:
		return redactToken(value)
	case FieldPassword:
		return "***"
	case FieldName:
		return redactName(value)
	case FieldAddress:
		return redactAddress(value)
	}
	return value
}

// RedactByKey automatically classifies and redacts a value.
func RedactByKey(key, value string) string {
	ft := Classify(key)
	if ft == FieldGeneral {
		return value
	}
	return Redact(ft, value)
}

func redactNIK(v string) string {
	if len(v) <= 4 {
		return "****"
	}
	return v[:4] + strings.Repeat("*", len(v)-4)
}

func redactEmail(v string) string {
	at := strings.IndexByte(v, '@')
	if at <= 0 {
		return "***@***"
	}
	local := v[:at]
	domain := v[at:]
	if len(local) <= 2 {
		return local[:1] + "***" + domain
	}
	return local[:2] + "***" + domain
}

func redactPhone(v string) string {
	if len(v) <= 4 {
		return "****"
	}
	return v[:3] + strings.Repeat("*", len(v)-7) + v[len(v)-4:]
}

func redactToken(v string) string {
	if len(v) <= 8 {
		return "****"
	}
	return v[:4] + "..." + v[len(v)-4:]
}

func redactName(v string) string {
	parts := strings.Fields(v)
	if len(parts) == 0 {
		return "***"
	}
	result := make([]string, len(parts))
	for i, p := range parts {
		if len(p) > 0 {
			result[i] = string(p[0]) + "***"
		}
	}
	return strings.Join(result, " ")
}

func redactAddress(v string) string {
	if len(v) <= 10 {
		return "***"
	}
	return v[:5] + "***"
}

// IsSensitive checks if a key name refers to a sensitive field.
func IsSensitive(key string) bool {
	return Classify(key) != FieldGeneral
}

// RegisterKey adds a custom sensitive key mapping.
func RegisterKey(key string, fieldType FieldType) {
	sensitiveKeys[strings.ToLower(key)] = fieldType
}
