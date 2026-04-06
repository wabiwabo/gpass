// Package errors provides structured error types for consistent API error responses
// across all GarudaPass services.
package errors

import (
	"fmt"
	"net/http"
)

// AppError represents a structured application error with an HTTP status code
// and machine-readable error code.
type AppError struct {
	Code       string `json:"error"`       // machine-readable: e.g. "invalid_request"
	Message    string `json:"message"`     // human-readable description
	HTTPStatus int    `json:"-"`           // HTTP status code
	Details    any    `json:"details,omitempty"` // optional structured details
}

func (e *AppError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Common error constructors

// BadRequest creates a 400 error.
func BadRequest(code, message string) *AppError {
	return &AppError{Code: code, Message: message, HTTPStatus: http.StatusBadRequest}
}

// Unauthorized creates a 401 error.
func Unauthorized(code, message string) *AppError {
	return &AppError{Code: code, Message: message, HTTPStatus: http.StatusUnauthorized}
}

// Forbidden creates a 403 error.
func Forbidden(code, message string) *AppError {
	return &AppError{Code: code, Message: message, HTTPStatus: http.StatusForbidden}
}

// NotFound creates a 404 error.
func NotFound(code, message string) *AppError {
	return &AppError{Code: code, Message: message, HTTPStatus: http.StatusNotFound}
}

// Conflict creates a 409 error.
func Conflict(code, message string) *AppError {
	return &AppError{Code: code, Message: message, HTTPStatus: http.StatusConflict}
}

// Gone creates a 410 error.
func Gone(code, message string) *AppError {
	return &AppError{Code: code, Message: message, HTTPStatus: http.StatusGone}
}

// TooLarge creates a 413 error.
func TooLarge(code, message string) *AppError {
	return &AppError{Code: code, Message: message, HTTPStatus: http.StatusRequestEntityTooLarge}
}

// TooManyRequests creates a 429 error.
func TooManyRequests(code, message string) *AppError {
	return &AppError{Code: code, Message: message, HTTPStatus: http.StatusTooManyRequests}
}

// Internal creates a 500 error.
func Internal(code, message string) *AppError {
	return &AppError{Code: code, Message: message, HTTPStatus: http.StatusInternalServerError}
}

// BadGateway creates a 502 error.
func BadGateway(code, message string) *AppError {
	return &AppError{Code: code, Message: message, HTTPStatus: http.StatusBadGateway}
}

// ServiceUnavailable creates a 503 error.
func ServiceUnavailable(code, message string) *AppError {
	return &AppError{Code: code, Message: message, HTTPStatus: http.StatusServiceUnavailable}
}

// WithDetails adds structured details to the error.
func (e *AppError) WithDetails(details any) *AppError {
	e.Details = details
	return e
}

// Standard error codes used across GarudaPass services.
const (
	// Authentication & Authorization
	CodeInvalidCredentials = "invalid_credentials"
	CodeSessionExpired     = "session_expired"
	CodeInsufficientAuth   = "insufficient_auth_level"
	CodeNotOwner           = "not_owner"
	CodeAccessDenied       = "access_denied"

	// Validation
	CodeInvalidRequest    = "invalid_request"
	CodeInvalidJSON       = "invalid_json"
	CodeMissingField      = "missing_required_field"
	CodeInvalidFormat     = "invalid_format"
	CodeInvalidNIK        = "invalid_nik"

	// Resources
	CodeResourceNotFound  = "resource_not_found"
	CodeAlreadyExists     = "already_exists"
	CodeResourceExpired   = "resource_expired"

	// Rate Limiting
	CodeRateLimitExceeded = "rate_limit_exceeded"
	CodeDailyLimitReached = "daily_limit_reached"

	// External Services
	CodeServiceUnavailable = "service_unavailable"
	CodeCircuitBreakerOpen = "circuit_breaker_open"
	CodeUpstreamError      = "upstream_error"

	// Signing
	CodeCertNotActive      = "certificate_not_active"
	CodeDocAlreadySigned   = "document_already_signed"
	CodeInvalidFileType    = "invalid_file_type"
	CodeFileTooLarge       = "file_too_large"

	// Corporate
	CodeEntityNotVerified  = "entity_not_verified"
	CodeRoleHierarchy      = "role_hierarchy_violation"
	CodeNotOfficer         = "not_registered_officer"
)
