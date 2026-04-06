package logging

import (
	"context"
	"log/slog"
	"os"
	"regexp"
	"strings"
)

// defaultSensitiveKeys are key names whose values should always be redacted.
var defaultSensitiveKeys = []string{
	"nik",
	"password",
	"secret",
	"token",
	"authorization",
	"cookie",
	"session_key",
	"api_key",
	"credit_card",
	"ssn",
	"pin",
}

// defaultValuePatterns are regex patterns that match sensitive values.
var defaultValuePatterns = []*regexp.Regexp{
	regexp.MustCompile(`\b\d{16}\b`),                                // NIK / 16-digit number
	regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`), // Email
	regexp.MustCompile(`\+?\d{10,15}`),                              // Phone
	regexp.MustCompile(`Bearer\s+\S+`),                              // Bearer token
	regexp.MustCompile(`\b\d{4}[\s\-]?\d{4}[\s\-]?\d{4}[\s\-]?\d{4}\b`), // Credit card
}

const redactedValue = "[REDACTED]"

// RedactingConfig allows customisation of the redacting handler.
type RedactingConfig struct {
	RedactKeys     []string         // additional keys to redact beyond defaults
	RedactPatterns []*regexp.Regexp  // additional value patterns to redact
}

// RedactingHandler is an slog.Handler that redacts sensitive data from log records.
type RedactingHandler struct {
	inner          slog.Handler
	sensitiveKeys  map[string]struct{}
	valuePatterns  []*regexp.Regexp
}

// NewRedactingHandler wraps inner with PII-redacting behaviour.
// If cfg is nil, only the default keys and patterns are used.
func NewRedactingHandler(inner slog.Handler, cfg *RedactingConfig) *RedactingHandler {
	keys := make(map[string]struct{}, len(defaultSensitiveKeys))
	for _, k := range defaultSensitiveKeys {
		keys[strings.ToLower(k)] = struct{}{}
	}

	patterns := make([]*regexp.Regexp, len(defaultValuePatterns))
	copy(patterns, defaultValuePatterns)

	if cfg != nil {
		for _, k := range cfg.RedactKeys {
			keys[strings.ToLower(k)] = struct{}{}
		}
		patterns = append(patterns, cfg.RedactPatterns...)
	}

	return &RedactingHandler{
		inner:         inner,
		sensitiveKeys: keys,
		valuePatterns: patterns,
	}
}

// Enabled delegates to the inner handler.
func (h *RedactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle redacts sensitive attributes before delegating to the inner handler.
func (h *RedactingHandler) Handle(ctx context.Context, r slog.Record) error {
	// Build a new record with redacted attrs.
	newRecord := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		newRecord.AddAttrs(h.redactAttr(a))
		return true
	})
	return h.inner.Handle(ctx, newRecord)
}

// WithAttrs returns a new handler with the given attrs redacted.
func (h *RedactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		redacted[i] = h.redactAttr(a)
	}
	return &RedactingHandler{
		inner:         h.inner.WithAttrs(redacted),
		sensitiveKeys: h.sensitiveKeys,
		valuePatterns: h.valuePatterns,
	}
}

// WithGroup returns a new handler with the given group.
func (h *RedactingHandler) WithGroup(name string) slog.Handler {
	return &RedactingHandler{
		inner:         h.inner.WithGroup(name),
		sensitiveKeys: h.sensitiveKeys,
		valuePatterns: h.valuePatterns,
	}
}

// redactAttr checks a single attribute and redacts if necessary.
func (h *RedactingHandler) redactAttr(a slog.Attr) slog.Attr {
	// Handle groups recursively.
	if a.Value.Kind() == slog.KindGroup {
		attrs := a.Value.Group()
		redacted := make([]slog.Attr, len(attrs))
		for i, ga := range attrs {
			redacted[i] = h.redactAttr(ga)
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(redacted...)}
	}

	// Check key name (case-insensitive).
	if _, ok := h.sensitiveKeys[strings.ToLower(a.Key)]; ok {
		return slog.String(a.Key, redactedValue)
	}

	// Check value against patterns.
	if a.Value.Kind() == slog.KindString {
		val := a.Value.String()
		for _, p := range h.valuePatterns {
			if p.MatchString(val) {
				return slog.String(a.Key, redactedValue)
			}
		}
	}

	return a
}

// SetupWithRedaction is like Setup but wraps the handler with a RedactingHandler.
// It uses default redacting configuration.
func SetupWithRedaction(cfg Config) func() {
	level := ParseLevel(cfg.Level)

	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: level}

	switch strings.ToLower(cfg.Format) {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	default:
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	// Wrap with redacting handler.
	handler = NewRedactingHandler(handler, nil)

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
