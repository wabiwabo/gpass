// Package apiresponse provides RFC 7807 Problem Details and standard API response
// helpers for consistent HTTP responses across all GarudaPass services.
package apiresponse

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/garudapass/gpass/packages/golib/errors"
)

// Problem represents an RFC 7807 Problem Details object.
type Problem struct {
	Type       string                 `json:"type"`
	Title      string                 `json:"title"`
	Status     int                    `json:"status"`
	Detail     string                 `json:"detail"`
	Instance   string                 `json:"instance,omitempty"`
	Extensions map[string]interface{} `json:"-"`
}

// MarshalJSON implements custom JSON marshaling to inline extensions.
func (p *Problem) MarshalJSON() ([]byte, error) {
	type plain Problem
	data, err := json.Marshal((*plain)(p))
	if err != nil {
		return nil, err
	}
	if len(p.Extensions) == 0 {
		return data, nil
	}

	// Merge extensions into the top-level object.
	var base map[string]interface{}
	if err := json.Unmarshal(data, &base); err != nil {
		return nil, err
	}
	for k, v := range p.Extensions {
		base[k] = v
	}
	return json.Marshal(base)
}

// NewProblem creates a new Problem with the given status, title, and detail.
// The Type field defaults to "about:blank" per RFC 7807.
func NewProblem(status int, title, detail string) *Problem {
	return &Problem{
		Type:   "about:blank",
		Title:  title,
		Status: status,
		Detail: detail,
	}
}

// WriteProblem writes a Problem as JSON with the application/problem+json
// Content-Type and the appropriate HTTP status code.
func WriteProblem(w http.ResponseWriter, p *Problem) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	json.NewEncoder(w).Encode(p)
}

// Success represents a standard success response.
type Success struct {
	Data interface{} `json:"data"`
	Meta *Meta       `json:"meta,omitempty"`
}

// Meta holds pagination metadata for list responses.
type Meta struct {
	Page       int    `json:"page"`
	PerPage    int    `json:"per_page"`
	TotalCount int    `json:"total_count"`
	TotalPages int    `json:"total_pages"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// WriteSuccess writes a 200 OK response with the given data and optional meta.
func WriteSuccess(w http.ResponseWriter, data interface{}, meta *Meta) {
	resp := Success{Data: data, Meta: meta}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// WriteCreated writes a 201 Created response with the given data.
func WriteCreated(w http.ResponseWriter, data interface{}) {
	resp := Success{Data: data}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// WriteNoContent writes a 204 No Content response with an empty body.
func WriteNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// WriteList writes a list response with pagination metadata.
func WriteList(w http.ResponseWriter, items interface{}, meta Meta) {
	resp := Success{Data: items, Meta: &meta}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// FromAppError converts an AppError to an RFC 7807 Problem.
func FromAppError(err *errors.AppError) *Problem {
	return &Problem{
		Type:   "about:blank",
		Title:  err.Code,
		Status: err.HTTPStatus,
		Detail: err.Message,
	}
}

// ValidationProblem creates a 422 Unprocessable Entity problem with field-level
// validation errors.
func ValidationProblem(detail string, fields map[string]string) *Problem {
	return &Problem{
		Type:   "about:blank",
		Title:  "Validation Error",
		Status: http.StatusUnprocessableEntity,
		Detail: detail,
		Extensions: map[string]interface{}{
			"fields": fields,
		},
	}
}

// NotFoundProblem creates a 404 Not Found problem for the given resource.
func NotFoundProblem(resource string) *Problem {
	return &Problem{
		Type:   "about:blank",
		Title:  "Not Found",
		Status: http.StatusNotFound,
		Detail: fmt.Sprintf("%s not found", resource),
	}
}

// RateLimitProblem creates a 429 Too Many Requests problem. When written with
// WriteProblem, the Retry-After header is set to retryAfter seconds.
func RateLimitProblem(retryAfter int) *Problem {
	return &Problem{
		Type:   "about:blank",
		Title:  "Too Many Requests",
		Status: http.StatusTooManyRequests,
		Detail: fmt.Sprintf("rate limit exceeded, retry after %d seconds", retryAfter),
		Extensions: map[string]interface{}{
			"retry_after": retryAfter,
		},
	}
}

// WriteRateLimitProblem writes a rate-limit problem with the Retry-After header.
func WriteRateLimitProblem(w http.ResponseWriter, retryAfter int) {
	p := RateLimitProblem(retryAfter)
	w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	WriteProblem(w, p)
}
