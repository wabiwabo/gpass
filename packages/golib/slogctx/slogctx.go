// Package slogctx provides context-aware slog integration.
// Stores and retrieves slog.Logger from context for request-scoped
// structured logging with automatic attribute propagation.
package slogctx

import (
	"context"
	"log/slog"
)

type contextKey int

const loggerKey contextKey = iota

// With stores a logger in the context.
func With(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// From retrieves the logger from context, or returns slog.Default().
func From(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey).(*slog.Logger); ok && logger != nil {
		return logger
	}
	return slog.Default()
}

// WithAttrs stores a logger with additional attributes in the context.
func WithAttrs(ctx context.Context, attrs ...slog.Attr) context.Context {
	logger := From(ctx)
	args := make([]any, len(attrs))
	for i, a := range attrs {
		args[i] = a
	}
	return With(ctx, logger.With(args...))
}

// WithRequestID adds a request_id attribute to the context logger.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return WithAttrs(ctx, slog.String("request_id", requestID))
}

// WithUserID adds a user_id attribute to the context logger.
func WithUserID(ctx context.Context, userID string) context.Context {
	return WithAttrs(ctx, slog.String("user_id", userID))
}

// WithService adds a service attribute to the context logger.
func WithService(ctx context.Context, service string) context.Context {
	return WithAttrs(ctx, slog.String("service", service))
}

// Info logs at INFO level using the context logger.
func Info(ctx context.Context, msg string, args ...any) {
	From(ctx).InfoContext(ctx, msg, args...)
}

// Error logs at ERROR level using the context logger.
func Error(ctx context.Context, msg string, args ...any) {
	From(ctx).ErrorContext(ctx, msg, args...)
}

// Warn logs at WARN level using the context logger.
func Warn(ctx context.Context, msg string, args ...any) {
	From(ctx).WarnContext(ctx, msg, args...)
}

// Debug logs at DEBUG level using the context logger.
func Debug(ctx context.Context, msg string, args ...any) {
	From(ctx).DebugContext(ctx, msg, args...)
}
