package logging

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"sync/atomic"
)

// SamplingConfig configures log sampling rates per level.
// Rate is a float64 between 0 and 1 (0 = drop all, 1 = keep all).
type SamplingConfig struct {
	DebugRate float64 // Default: 0.01 (1%)
	InfoRate  float64 // Default: 0.1 (10%)
	WarnRate  float64 // Default: 1.0 (100%)
	ErrorRate float64 // Default: 1.0 (100%)
}

// DefaultSamplingConfig returns enterprise defaults:
// 100% errors/warns, 10% info, 1% debug.
func DefaultSamplingConfig() SamplingConfig {
	return SamplingConfig{
		DebugRate: 0.01,
		InfoRate:  0.1,
		WarnRate:  1.0,
		ErrorRate: 1.0,
	}
}

// SamplingHandler wraps a slog.Handler and samples logs by level.
// This reduces log volume for high-throughput services while
// preserving all error and warning signals.
type SamplingHandler struct {
	inner  slog.Handler
	config SamplingConfig

	// Metrics
	total   atomic.Int64
	sampled atomic.Int64
	dropped atomic.Int64
}

// NewSamplingHandler creates a handler that samples logs by level.
func NewSamplingHandler(inner slog.Handler, cfg SamplingConfig) *SamplingHandler {
	if cfg.ErrorRate == 0 {
		cfg.ErrorRate = 1.0
	}
	if cfg.WarnRate == 0 {
		cfg.WarnRate = 1.0
	}
	return &SamplingHandler{
		inner:  inner,
		config: cfg,
	}
}

// Enabled delegates to the inner handler.
func (h *SamplingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle samples the log record based on its level.
func (h *SamplingHandler) Handle(ctx context.Context, r slog.Record) error {
	h.total.Add(1)

	rate := h.rateForLevel(r.Level)
	if rate >= 1.0 || rand.Float64() < rate {
		h.sampled.Add(1)
		return h.inner.Handle(ctx, r)
	}

	h.dropped.Add(1)
	return nil
}

// WithAttrs returns a new handler with the given attributes.
func (h *SamplingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &SamplingHandler{
		inner:  h.inner.WithAttrs(attrs),
		config: h.config,
	}
}

// WithGroup returns a new handler with the given group name.
func (h *SamplingHandler) WithGroup(name string) slog.Handler {
	return &SamplingHandler{
		inner:  h.inner.WithGroup(name),
		config: h.config,
	}
}

func (h *SamplingHandler) rateForLevel(level slog.Level) float64 {
	switch {
	case level >= slog.LevelError:
		return h.config.ErrorRate
	case level >= slog.LevelWarn:
		return h.config.WarnRate
	case level >= slog.LevelInfo:
		return h.config.InfoRate
	default:
		return h.config.DebugRate
	}
}

// SamplingStats holds sampling statistics.
type SamplingStats struct {
	Total   int64   `json:"total"`
	Sampled int64   `json:"sampled"`
	Dropped int64   `json:"dropped"`
	Rate    float64 `json:"effective_rate"`
}

// Stats returns current sampling statistics.
func (h *SamplingHandler) Stats() SamplingStats {
	total := h.total.Load()
	sampled := h.sampled.Load()
	dropped := h.dropped.Load()

	rate := 0.0
	if total > 0 {
		rate = float64(sampled) / float64(total)
	}

	return SamplingStats{
		Total:   total,
		Sampled: sampled,
		Dropped: dropped,
		Rate:    rate,
	}
}
