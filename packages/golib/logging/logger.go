package logging

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

type ctxKey string

const (
	ctxRequestID     ctxKey = "request_id"
	ctxUserID        ctxKey = "user_id"
	ctxCorrelationID ctxKey = "correlation_id"
)

// Config for the logger.
type Config struct {
	ServiceName string
	Environment string // development, staging, production
	Level       string // debug, info, warn, error
	Format      string // json, text
}

// Fields is a convenience type for structured log fields.
type Fields map[string]interface{}

// Setup initializes the global slog logger with enterprise configuration.
// Returns a cleanup function.
func Setup(cfg Config) func() {
	level := ParseLevel(cfg.Level)

	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: level}

	switch strings.ToLower(cfg.Format) {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	default:
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	// Add default attributes for service context.
	attrs := []slog.Attr{}
	if cfg.ServiceName != "" {
		attrs = append(attrs, slog.String("service", cfg.ServiceName))
	}
	if cfg.Environment != "" {
		attrs = append(attrs, slog.String("environment", cfg.Environment))
	}
	if len(attrs) > 0 {
		handler = handler.WithAttrs(attrs)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return func() {
		// Cleanup: flush or close resources if needed in the future.
	}
}

// ParseLevel parses a log level string. Defaults to Info for unknown values.
func ParseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// FromContext returns a logger enriched with context values (request_id, user_id, correlation_id).
func FromContext(ctx context.Context) *slog.Logger {
	logger := slog.Default()

	if ctx == nil {
		return logger
	}

	if v, ok := ctx.Value(ctxRequestID).(string); ok && v != "" {
		logger = logger.With("request_id", v)
	}
	if v, ok := ctx.Value(ctxUserID).(string); ok && v != "" {
		logger = logger.With("user_id", v)
	}
	if v, ok := ctx.Value(ctxCorrelationID).(string); ok && v != "" {
		logger = logger.With("correlation_id", v)
	}

	return logger
}

// WithService returns a logger with service name attribute.
func WithService(name string) *slog.Logger {
	return slog.Default().With("service", name)
}

// WithRequestID adds a request ID to the context for later retrieval by FromContext.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxRequestID, id)
}

// WithUserID adds a user ID to the context for later retrieval by FromContext.
func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxUserID, id)
}

// WithCorrelationID adds a correlation ID to the context for later retrieval by FromContext.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxCorrelationID, id)
}

// Error logs at error level with fields.
func Error(ctx context.Context, msg string, fields Fields) {
	FromContext(ctx).Error(msg, fieldsToAttrs(fields)...)
}

// Warn logs at warn level with fields.
func Warn(ctx context.Context, msg string, fields Fields) {
	FromContext(ctx).Warn(msg, fieldsToAttrs(fields)...)
}

// Info logs at info level with fields.
func Info(ctx context.Context, msg string, fields Fields) {
	FromContext(ctx).Info(msg, fieldsToAttrs(fields)...)
}

// Debug logs at debug level with fields.
func Debug(ctx context.Context, msg string, fields Fields) {
	FromContext(ctx).Debug(msg, fieldsToAttrs(fields)...)
}

func fieldsToAttrs(fields Fields) []any {
	if len(fields) == 0 {
		return nil
	}
	attrs := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		attrs = append(attrs, k, v)
	}
	return attrs
}
