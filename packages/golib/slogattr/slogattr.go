// Package slogattr provides slog attribute helpers for consistent
// structured logging across services. Pre-defines common attribute
// constructors for request, user, and error contexts.
package slogattr

import (
	"log/slog"
	"net/http"
	"time"
)

// Request returns slog attributes for an HTTP request.
func Request(r *http.Request) []slog.Attr {
	attrs := []slog.Attr{
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.String("remote_addr", r.RemoteAddr),
	}
	if r.URL.RawQuery != "" {
		attrs = append(attrs, slog.String("query", r.URL.RawQuery))
	}
	if ua := r.UserAgent(); ua != "" {
		attrs = append(attrs, slog.String("user_agent", ua))
	}
	return attrs
}

// Error returns a slog attribute for an error.
func Error(err error) slog.Attr {
	if err == nil {
		return slog.String("error", "")
	}
	return slog.String("error", err.Error())
}

// UserID returns a user ID attribute.
func UserID(id string) slog.Attr {
	return slog.String("user_id", id)
}

// TenantID returns a tenant ID attribute.
func TenantID(id string) slog.Attr {
	return slog.String("tenant_id", id)
}

// RequestID returns a request ID attribute.
func RequestID(id string) slog.Attr {
	return slog.String("request_id", id)
}

// TraceID returns a trace ID attribute.
func TraceID(id string) slog.Attr {
	return slog.String("trace_id", id)
}

// Duration returns a duration attribute.
func Duration(d time.Duration) slog.Attr {
	return slog.Duration("duration", d)
}

// Status returns an HTTP status attribute.
func Status(code int) slog.Attr {
	return slog.Int("status", code)
}

// Service returns a service name attribute.
func Service(name string) slog.Attr {
	return slog.String("service", name)
}

// Operation returns an operation name attribute.
func Operation(name string) slog.Attr {
	return slog.String("operation", name)
}

// Resource returns resource type and ID attributes.
func Resource(resourceType, resourceID string) slog.Attr {
	return slog.Group("resource",
		slog.String("type", resourceType),
		slog.String("id", resourceID),
	)
}

// Count returns a count attribute.
func Count(n int) slog.Attr {
	return slog.Int("count", n)
}
