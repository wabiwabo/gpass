// Package httpstatus provides HTTP status code utilities.
// Classifies status codes and provides human-readable descriptions
// for common API status patterns.
package httpstatus

// IsSuccess checks if a status code indicates success (2xx).
func IsSuccess(code int) bool {
	return code >= 200 && code < 300
}

// IsRedirect checks if a status code indicates redirection (3xx).
func IsRedirect(code int) bool {
	return code >= 300 && code < 400
}

// IsClientError checks if a status code indicates a client error (4xx).
func IsClientError(code int) bool {
	return code >= 400 && code < 500
}

// IsServerError checks if a status code indicates a server error (5xx).
func IsServerError(code int) bool {
	return code >= 500 && code < 600
}

// IsError checks if a status code indicates any error (4xx or 5xx).
func IsError(code int) bool {
	return code >= 400
}

// IsRetryable checks if a status code indicates a retryable error.
func IsRetryable(code int) bool {
	switch code {
	case 408, 429, 500, 502, 503, 504:
		return true
	}
	return false
}

// Category returns the category name for a status code.
func Category(code int) string {
	switch {
	case code >= 100 && code < 200:
		return "informational"
	case code >= 200 && code < 300:
		return "success"
	case code >= 300 && code < 400:
		return "redirection"
	case code >= 400 && code < 500:
		return "client_error"
	case code >= 500 && code < 600:
		return "server_error"
	default:
		return "unknown"
	}
}

// Description returns a machine-readable description for common codes.
func Description(code int) string {
	switch code {
	case 200:
		return "ok"
	case 201:
		return "created"
	case 204:
		return "no_content"
	case 301:
		return "moved_permanently"
	case 302:
		return "found"
	case 304:
		return "not_modified"
	case 400:
		return "bad_request"
	case 401:
		return "unauthorized"
	case 403:
		return "forbidden"
	case 404:
		return "not_found"
	case 405:
		return "method_not_allowed"
	case 408:
		return "request_timeout"
	case 409:
		return "conflict"
	case 422:
		return "unprocessable_entity"
	case 429:
		return "too_many_requests"
	case 500:
		return "internal_server_error"
	case 502:
		return "bad_gateway"
	case 503:
		return "service_unavailable"
	case 504:
		return "gateway_timeout"
	default:
		return Category(code)
	}
}
