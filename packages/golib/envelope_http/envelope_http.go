// Package envelope_http provides standardized HTTP response
// envelope formatting. Wraps API responses in a consistent
// structure with metadata, pagination, and error information.
package envelope_http

import (
	"encoding/json"
	"net/http"
	"time"
)

// Response is the standard API response envelope.
type Response struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     *ErrorInfo  `json:"error,omitempty"`
	Meta      *Meta       `json:"meta,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
}

// ErrorInfo describes an API error.
type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// Meta contains response metadata.
type Meta struct {
	Timestamp  time.Time   `json:"timestamp"`
	Version    string      `json:"version,omitempty"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

// Pagination contains pagination info.
type Pagination struct {
	Page       int  `json:"page"`
	PerPage    int  `json:"per_page"`
	Total      int  `json:"total"`
	TotalPages int  `json:"total_pages"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}

// OK writes a success response.
func OK(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    data,
		Meta:    &Meta{Timestamp: time.Now().UTC()},
	})
}

// Created writes a 201 success response.
func Created(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusCreated, Response{
		Success: true,
		Data:    data,
		Meta:    &Meta{Timestamp: time.Now().UTC()},
	})
}

// NoContent writes a 204 response with no body.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Paginated writes a success response with pagination metadata.
func Paginated(w http.ResponseWriter, data interface{}, page, perPage, total int) {
	totalPages := total / perPage
	if total%perPage > 0 {
		totalPages++
	}

	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    data,
		Meta: &Meta{
			Timestamp: time.Now().UTC(),
			Pagination: &Pagination{
				Page:       page,
				PerPage:    perPage,
				Total:      total,
				TotalPages: totalPages,
				HasNext:    page < totalPages,
				HasPrev:    page > 1,
			},
		},
	})
}

// Error writes an error response.
func Error(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, Response{
		Success: false,
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
		},
		Meta: &Meta{Timestamp: time.Now().UTC()},
	})
}

// ErrorWithDetail writes an error response with additional detail.
func ErrorWithDetail(w http.ResponseWriter, status int, code, message, detail string) {
	writeJSON(w, status, Response{
		Success: false,
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
			Detail:  detail,
		},
		Meta: &Meta{Timestamp: time.Now().UTC()},
	})
}

// WithRequestID adds the request ID to the response.
func WithRequestID(w http.ResponseWriter, status int, resp Response, requestID string) {
	resp.RequestID = requestID
	writeJSON(w, status, resp)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
