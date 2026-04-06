// Package structured provides structured error reporting for API consumers.
// It extends RFC 7807 Problem Details with field-level errors, error codes,
// and machine-readable metadata for frontend consumption.
package structured

import (
	"encoding/json"
	"net/http"
)

// Error represents a structured API error response.
type Error struct {
	Type     string            `json:"type"`               // URI identifying the error type.
	Title    string            `json:"title"`               // Human-readable summary.
	Status   int               `json:"status"`              // HTTP status code.
	Detail   string            `json:"detail,omitempty"`    // Human-readable explanation.
	Instance string            `json:"instance,omitempty"`  // URI of the specific occurrence.
	Code     string            `json:"code,omitempty"`      // Machine-readable error code.
	Fields   []FieldError      `json:"fields,omitempty"`    // Field-level validation errors.
	Meta     map[string]string `json:"meta,omitempty"`      // Additional metadata.
}

// FieldError represents a validation error on a specific field.
type FieldError struct {
	Field   string `json:"field"`   // Field name (dot-notation for nested).
	Code    string `json:"code"`    // Machine-readable error code.
	Message string `json:"message"` // Human-readable description.
}

// Builder provides fluent error construction.
type Builder struct {
	err Error
}

// NewError creates a new error builder.
func NewError(status int, title string) *Builder {
	return &Builder{
		err: Error{
			Type:   "about:blank",
			Title:  title,
			Status: status,
		},
	}
}

// BadRequest creates a 400 error builder.
func BadRequest(title string) *Builder {
	return NewError(http.StatusBadRequest, title)
}

// NotFound creates a 404 error builder.
func NotFound(title string) *Builder {
	return NewError(http.StatusNotFound, title)
}

// Forbidden creates a 403 error builder.
func Forbidden(title string) *Builder {
	return NewError(http.StatusForbidden, title)
}

// Unauthorized creates a 401 error builder.
func Unauthorized(title string) *Builder {
	return NewError(http.StatusUnauthorized, title)
}

// Conflict creates a 409 error builder.
func Conflict(title string) *Builder {
	return NewError(http.StatusConflict, title)
}

// InternalError creates a 500 error builder.
func InternalError(title string) *Builder {
	return NewError(http.StatusInternalServerError, title)
}

// Type sets the error type URI.
func (b *Builder) Type(uri string) *Builder {
	b.err.Type = uri
	return b
}

// Detail sets the detail message.
func (b *Builder) Detail(detail string) *Builder {
	b.err.Detail = detail
	return b
}

// Instance sets the instance URI.
func (b *Builder) Instance(instance string) *Builder {
	b.err.Instance = instance
	return b
}

// Code sets the machine-readable error code.
func (b *Builder) Code(code string) *Builder {
	b.err.Code = code
	return b
}

// Field adds a field-level error.
func (b *Builder) Field(field, code, message string) *Builder {
	b.err.Fields = append(b.err.Fields, FieldError{
		Field:   field,
		Code:    code,
		Message: message,
	})
	return b
}

// Meta adds metadata key-value pair.
func (b *Builder) Meta(key, value string) *Builder {
	if b.err.Meta == nil {
		b.err.Meta = make(map[string]string)
	}
	b.err.Meta[key] = value
	return b
}

// Build returns the constructed error.
func (b *Builder) Build() Error {
	return b.err
}

// Write writes the error as an HTTP response.
func (b *Builder) Write(w http.ResponseWriter) {
	WriteError(w, b.err)
}

// WriteError writes a structured error response.
func WriteError(w http.ResponseWriter, err Error) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(err.Status)
	json.NewEncoder(w).Encode(err)
}

// HasFields checks if the error has field-level errors.
func (e Error) HasFields() bool {
	return len(e.Fields) > 0
}

// FieldCount returns the number of field errors.
func (e Error) FieldCount() int {
	return len(e.Fields)
}

// Error implements the error interface.
func (e Error) Error() string {
	if e.Detail != "" {
		return e.Title + ": " + e.Detail
	}
	return e.Title
}
