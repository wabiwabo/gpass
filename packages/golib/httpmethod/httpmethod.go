// Package httpmethod provides HTTP method constants and utilities.
// Validates and categorizes HTTP methods for routing and middleware.
package httpmethod

import "net/http"

// Standard HTTP methods.
const (
	GET     = http.MethodGet
	POST    = http.MethodPost
	PUT     = http.MethodPut
	PATCH   = http.MethodPatch
	DELETE  = http.MethodDelete
	HEAD    = http.MethodHead
	OPTIONS = http.MethodOptions
	TRACE   = http.MethodTrace
	CONNECT = http.MethodConnect
)

var validMethods = map[string]bool{
	GET: true, POST: true, PUT: true, PATCH: true,
	DELETE: true, HEAD: true, OPTIONS: true,
	TRACE: true, CONNECT: true,
}

// IsValid checks if a method is a standard HTTP method.
func IsValid(method string) bool {
	return validMethods[method]
}

// IsSafe checks if a method is safe (read-only, no side effects).
func IsSafe(method string) bool {
	switch method {
	case GET, HEAD, OPTIONS, TRACE:
		return true
	}
	return false
}

// IsIdempotent checks if a method is idempotent.
func IsIdempotent(method string) bool {
	switch method {
	case GET, HEAD, PUT, DELETE, OPTIONS, TRACE:
		return true
	}
	return false
}

// HasBody checks if a method typically has a request body.
func HasBody(method string) bool {
	switch method {
	case POST, PUT, PATCH:
		return true
	}
	return false
}

// IsCacheable checks if responses to a method can be cached.
func IsCacheable(method string) bool {
	switch method {
	case GET, HEAD:
		return true
	}
	return false
}

// All returns all standard HTTP methods.
func All() []string {
	return []string{GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS, TRACE, CONNECT}
}

// Safe returns all safe HTTP methods.
func Safe() []string {
	return []string{GET, HEAD, OPTIONS, TRACE}
}

// Unsafe returns all unsafe (mutating) HTTP methods.
func Unsafe() []string {
	return []string{POST, PUT, PATCH, DELETE, CONNECT}
}
