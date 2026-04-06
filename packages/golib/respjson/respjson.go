// Package respjson provides JSON response writing helpers.
// Handles content-type headers, status codes, and error
// formatting for consistent API responses.
package respjson

import (
	"encoding/json"
	"net/http"
)

// OK writes a 200 JSON response.
func OK(w http.ResponseWriter, v interface{}) {
	write(w, http.StatusOK, v)
}

// Created writes a 201 JSON response.
func Created(w http.ResponseWriter, v interface{}) {
	write(w, http.StatusCreated, v)
}

// NoContent writes a 204 response.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Write writes a JSON response with the given status code.
func Write(w http.ResponseWriter, status int, v interface{}) {
	write(w, status, v)
}

// Error writes a JSON error response.
func Error(w http.ResponseWriter, status int, code, message string) {
	write(w, status, map[string]interface{}{
		"error":   code,
		"message": message,
		"status":  status,
	})
}

// ErrorWithDetail writes a JSON error with detail.
func ErrorWithDetail(w http.ResponseWriter, status int, code, message, detail string) {
	write(w, status, map[string]interface{}{
		"error":   code,
		"message": message,
		"detail":  detail,
		"status":  status,
	})
}

// BadRequest writes a 400 error.
func BadRequest(w http.ResponseWriter, code, message string) {
	Error(w, http.StatusBadRequest, code, message)
}

// Unauthorized writes a 401 error.
func Unauthorized(w http.ResponseWriter, message string) {
	Error(w, http.StatusUnauthorized, "unauthorized", message)
}

// Forbidden writes a 403 error.
func Forbidden(w http.ResponseWriter, message string) {
	Error(w, http.StatusForbidden, "forbidden", message)
}

// NotFound writes a 404 error.
func NotFound(w http.ResponseWriter, message string) {
	Error(w, http.StatusNotFound, "not_found", message)
}

// Conflict writes a 409 error.
func Conflict(w http.ResponseWriter, message string) {
	Error(w, http.StatusConflict, "conflict", message)
}

// TooManyRequests writes a 429 error.
func TooManyRequests(w http.ResponseWriter, message string) {
	Error(w, http.StatusTooManyRequests, "rate_limited", message)
}

// Internal writes a 500 error.
func Internal(w http.ResponseWriter, message string) {
	Error(w, http.StatusInternalServerError, "internal_error", message)
}

// ServiceUnavailable writes a 503 error.
func ServiceUnavailable(w http.ResponseWriter, message string) {
	Error(w, http.StatusServiceUnavailable, "service_unavailable", message)
}

func write(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		json.NewEncoder(w).Encode(v)
	}
}
