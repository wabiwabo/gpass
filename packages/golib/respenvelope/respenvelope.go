// Package respenvelope provides a standardized API response envelope
// for consistent JSON response formatting across all services.
// Wraps success and error responses in a uniform structure.
package respenvelope

import (
	"encoding/json"
	"net/http"
	"time"
)

// Envelope wraps all API responses in a consistent format.
type Envelope struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     *ErrorInfo  `json:"error,omitempty"`
	Meta      *Meta       `json:"meta,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	RequestID string      `json:"request_id,omitempty"`
}

// ErrorInfo holds error details.
type ErrorInfo struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// Meta holds pagination and other metadata.
type Meta struct {
	Page       int   `json:"page,omitempty"`
	PerPage    int   `json:"per_page,omitempty"`
	Total      int   `json:"total,omitempty"`
	TotalPages int   `json:"total_pages,omitempty"`
	Count      int   `json:"count,omitempty"`
}

// OK writes a success response with data.
func OK(w http.ResponseWriter, data interface{}) {
	write(w, http.StatusOK, Envelope{
		Success:   true,
		Data:      data,
		Timestamp: time.Now(),
		RequestID: "", // Set by middleware.
	})
}

// OKWithMeta writes a success response with data and metadata.
func OKWithMeta(w http.ResponseWriter, data interface{}, meta Meta) {
	write(w, http.StatusOK, Envelope{
		Success:   true,
		Data:      data,
		Meta:      &meta,
		Timestamp: time.Now(),
	})
}

// Created writes a 201 response with data.
func Created(w http.ResponseWriter, data interface{}) {
	write(w, http.StatusCreated, Envelope{
		Success:   true,
		Data:      data,
		Timestamp: time.Now(),
	})
}

// NoContent writes a 204 response.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Err writes an error response.
func Err(w http.ResponseWriter, status int, code, message string) {
	write(w, status, Envelope{
		Success: false,
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
		},
		Timestamp: time.Now(),
	})
}

// ErrWithDetails writes an error response with additional details.
func ErrWithDetails(w http.ResponseWriter, status int, code, message string, details interface{}) {
	write(w, status, Envelope{
		Success: false,
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
			Details: details,
		},
		Timestamp: time.Now(),
	})
}

// BadRequest writes a 400 error.
func BadRequest(w http.ResponseWriter, code, message string) {
	Err(w, http.StatusBadRequest, code, message)
}

// NotFound writes a 404 error.
func NotFound(w http.ResponseWriter, message string) {
	Err(w, http.StatusNotFound, "not_found", message)
}

// Forbidden writes a 403 error.
func Forbidden(w http.ResponseWriter, message string) {
	Err(w, http.StatusForbidden, "forbidden", message)
}

// InternalError writes a 500 error.
func InternalError(w http.ResponseWriter, message string) {
	Err(w, http.StatusInternalServerError, "internal_error", message)
}

func write(w http.ResponseWriter, status int, env Envelope) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(env)
}
