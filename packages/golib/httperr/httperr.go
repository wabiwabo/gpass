// Package httperr provides HTTP error handling helpers that map
// application errors to appropriate HTTP responses with consistent
// formatting. Bridges the gap between internal errors and API responses.
package httperr

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// HTTPError represents an error with an HTTP status code.
type HTTPError struct {
	Status  int    `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// Error implements the error interface.
func (e *HTTPError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("[%d] %s: %s", e.Status, e.Code, e.Detail)
	}
	return fmt.Sprintf("[%d] %s: %s", e.Status, e.Code, e.Message)
}

// Write writes the error as an HTTP response.
func (e *HTTPError) Write(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(e.Status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"type":    "about:blank",
		"title":   e.Message,
		"status":  e.Status,
		"code":    e.Code,
		"detail":  e.Detail,
	})
}

// New creates an HTTPError.
func New(status int, code, message string) *HTTPError {
	return &HTTPError{Status: status, Code: code, Message: message}
}

// WithDetail adds detail to an HTTPError.
func (e *HTTPError) WithDetail(detail string) *HTTPError {
	e.Detail = detail
	return e
}

// Common error constructors.

func BadRequest(code, message string) *HTTPError {
	return New(http.StatusBadRequest, code, message)
}

func Unauthorized(code, message string) *HTTPError {
	return New(http.StatusUnauthorized, code, message)
}

func Forbidden(code, message string) *HTTPError {
	return New(http.StatusForbidden, code, message)
}

func NotFound(code, message string) *HTTPError {
	return New(http.StatusNotFound, code, message)
}

func Conflict(code, message string) *HTTPError {
	return New(http.StatusConflict, code, message)
}

func TooManyRequests(message string) *HTTPError {
	return New(http.StatusTooManyRequests, "rate_limited", message)
}

func Internal(message string) *HTTPError {
	return New(http.StatusInternalServerError, "internal_error", message)
}

func ServiceUnavailable(message string) *HTTPError {
	return New(http.StatusServiceUnavailable, "service_unavailable", message)
}

// Handle writes an error to the response if err is non-nil.
// Returns true if an error was handled.
func Handle(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}

	if httpErr, ok := err.(*HTTPError); ok {
		httpErr.Write(w)
		return true
	}

	// Default to 500 for unknown errors.
	Internal(err.Error()).Write(w)
	return true
}

// Must panics if err is non-nil (for init-time checks).
func Must(err error) {
	if err != nil {
		panic(err)
	}
}
