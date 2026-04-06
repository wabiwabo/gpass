// Package spanlog provides structured logging with trace context.
// Automatically injects trace_id, span_id, and service name into
// log entries for correlated distributed debugging.
package spanlog

import (
	"context"
	"log/slog"
)

type contextKey int

const keyTraceFields contextKey = iota

// Fields holds trace context fields for logging.
type Fields struct {
	TraceID   string
	SpanID    string
	Service   string
	RequestID string
}

// WithFields stores trace fields in context.
func WithFields(ctx context.Context, f Fields) context.Context {
	return context.WithValue(ctx, keyTraceFields, f)
}

// FromContext extracts trace fields from context.
func FromContext(ctx context.Context) Fields {
	f, _ := ctx.Value(keyTraceFields).(Fields)
	return f
}

// Logger returns a slog.Logger enriched with trace context.
func Logger(ctx context.Context, base *slog.Logger) *slog.Logger {
	if base == nil {
		base = slog.Default()
	}
	f := FromContext(ctx)

	attrs := make([]any, 0, 8)
	if f.TraceID != "" {
		attrs = append(attrs, slog.String("trace_id", f.TraceID))
	}
	if f.SpanID != "" {
		attrs = append(attrs, slog.String("span_id", f.SpanID))
	}
	if f.Service != "" {
		attrs = append(attrs, slog.String("service", f.Service))
	}
	if f.RequestID != "" {
		attrs = append(attrs, slog.String("request_id", f.RequestID))
	}

	if len(attrs) == 0 {
		return base
	}
	return base.With(attrs...)
}

// Info logs at info level with trace context.
func Info(ctx context.Context, msg string, args ...any) {
	Logger(ctx, nil).Info(msg, args...)
}

// Warn logs at warn level with trace context.
func Warn(ctx context.Context, msg string, args ...any) {
	Logger(ctx, nil).Warn(msg, args...)
}

// Error logs at error level with trace context.
func Error(ctx context.Context, msg string, args ...any) {
	Logger(ctx, nil).Error(msg, args...)
}

// Debug logs at debug level with trace context.
func Debug(ctx context.Context, msg string, args ...any) {
	Logger(ctx, nil).Debug(msg, args...)
}
