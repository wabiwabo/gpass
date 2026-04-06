// Package errcode provides a centralized error code registry for
// consistent error reporting across all GarudaPass services.
// Each error has a unique code, HTTP status, and default message.
package errcode

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

// Code represents a registered error code.
type Code struct {
	Code    string `json:"code"`
	Status  int    `json:"status"`
	Message string `json:"message"`
}

// Registry holds all registered error codes.
type Registry struct {
	mu    sync.RWMutex
	codes map[string]Code
}

// NewRegistry creates an error code registry.
func NewRegistry() *Registry {
	return &Registry{codes: make(map[string]Code)}
}

// Register adds an error code.
func (r *Registry) Register(code string, status int, message string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.codes[code] = Code{Code: code, Status: status, Message: message}
}

// Get retrieves an error code.
func (r *Registry) Get(code string) (Code, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.codes[code]
	return c, ok
}

// Write writes an error response using the registered code.
func (r *Registry) Write(w http.ResponseWriter, code string) {
	c, ok := r.Get(code)
	if !ok {
		c = Code{Code: code, Status: http.StatusInternalServerError, Message: "Unknown error"}
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(c.Status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"type":    "about:blank",
		"title":   c.Message,
		"status":  c.Status,
		"code":    c.Code,
	})
}

// WriteWithDetail writes an error with additional detail.
func (r *Registry) WriteWithDetail(w http.ResponseWriter, code, detail string) {
	c, ok := r.Get(code)
	if !ok {
		c = Code{Code: code, Status: http.StatusInternalServerError, Message: "Unknown error"}
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(c.Status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"type":    "about:blank",
		"title":   c.Message,
		"status":  c.Status,
		"code":    c.Code,
		"detail":  detail,
	})
}

// All returns all registered codes.
func (r *Registry) All() []Code {
	r.mu.RLock()
	defer r.mu.RUnlock()
	codes := make([]Code, 0, len(r.codes))
	for _, c := range r.codes {
		codes = append(codes, c)
	}
	return codes
}

// Count returns the number of registered codes.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.codes)
}

// Default returns a pre-populated registry with common error codes.
func Default() *Registry {
	r := NewRegistry()

	// Authentication.
	r.Register("auth_required", 401, "Authentication required")
	r.Register("auth_invalid", 401, "Invalid credentials")
	r.Register("auth_expired", 401, "Authentication expired")
	r.Register("token_invalid", 401, "Invalid token")

	// Authorization.
	r.Register("forbidden", 403, "Access denied")
	r.Register("insufficient_scope", 403, "Insufficient permissions")
	r.Register("tenant_mismatch", 403, "Tenant mismatch")

	// Validation.
	r.Register("validation_failed", 400, "Validation failed")
	r.Register("invalid_input", 400, "Invalid input")
	r.Register("missing_field", 400, "Required field missing")
	r.Register("invalid_format", 400, "Invalid format")

	// Resources.
	r.Register("not_found", 404, "Resource not found")
	r.Register("already_exists", 409, "Resource already exists")
	r.Register("conflict", 409, "Resource conflict")

	// Rate limiting.
	r.Register("rate_limited", 429, "Too many requests")
	r.Register("quota_exceeded", 429, "API quota exceeded")

	// Server.
	r.Register("internal_error", 500, "Internal server error")
	r.Register("service_unavailable", 503, "Service unavailable")
	r.Register("timeout", 504, "Gateway timeout")

	// Indonesian-specific.
	r.Register("nik_invalid", 400, "Invalid NIK format")
	r.Register("npwp_invalid", 400, "Invalid NPWP format")
	r.Register("nib_invalid", 400, "Invalid NIB format")
	r.Register("consent_required", 403, "Data consent required (UU PDP)")

	return r
}

// Error creates a standard error from a code.
func (c Code) Error() string {
	return fmt.Sprintf("[%s] %s", c.Code, c.Message)
}
