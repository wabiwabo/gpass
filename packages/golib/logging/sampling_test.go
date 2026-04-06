package logging

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
)

func TestSamplingHandler_ErrorsAlwaysLogged(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	h := NewSamplingHandler(inner, DefaultSamplingConfig())
	logger := slog.New(h)

	for i := 0; i < 100; i++ {
		logger.Error("critical error")
	}

	stats := h.Stats()
	if stats.Sampled != 100 {
		t.Errorf("errors should be 100%% sampled: got %d/100", stats.Sampled)
	}
}

func TestSamplingHandler_WarnsAlwaysLogged(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	h := NewSamplingHandler(inner, DefaultSamplingConfig())
	logger := slog.New(h)

	for i := 0; i < 100; i++ {
		logger.Warn("warning")
	}

	stats := h.Stats()
	if stats.Sampled != 100 {
		t.Errorf("warns should be 100%% sampled: got %d/100", stats.Sampled)
	}
}

func TestSamplingHandler_InfoSampled(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	h := NewSamplingHandler(inner, SamplingConfig{
		InfoRate:  0.5, // 50%
		ErrorRate: 1.0,
		WarnRate:  1.0,
	})
	logger := slog.New(h)

	for i := 0; i < 1000; i++ {
		logger.Info("info message")
	}

	stats := h.Stats()
	// With 50% sampling, we expect ~500 sampled, allow ±150 for randomness.
	if stats.Sampled < 300 || stats.Sampled > 700 {
		t.Errorf("50%% sampling: expected ~500, got %d", stats.Sampled)
	}
	if stats.Total != 1000 {
		t.Errorf("total: got %d, want 1000", stats.Total)
	}
}

func TestSamplingHandler_DebugHeavilySampled(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	h := NewSamplingHandler(inner, SamplingConfig{
		DebugRate: 0.01, // 1%
		InfoRate:  1.0,
		WarnRate:  1.0,
		ErrorRate: 1.0,
	})
	logger := slog.New(h)

	for i := 0; i < 10000; i++ {
		logger.Debug("verbose debug")
	}

	stats := h.Stats()
	// 1% of 10000 = ~100, allow ±80.
	if stats.Sampled > 250 {
		t.Errorf("1%% debug sampling: expected ~100, got %d", stats.Sampled)
	}
}

func TestSamplingHandler_ZeroRateDropsAll(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	h := NewSamplingHandler(inner, SamplingConfig{
		DebugRate: 0,
		InfoRate:  0,
		WarnRate:  1.0,
		ErrorRate: 1.0,
	})
	logger := slog.New(h)

	for i := 0; i < 100; i++ {
		logger.Info("dropped")
	}

	stats := h.Stats()
	if stats.Sampled != 0 {
		t.Errorf("0%% rate should drop all: got %d sampled", stats.Sampled)
	}
	if stats.Dropped != 100 {
		t.Errorf("dropped: got %d, want 100", stats.Dropped)
	}
}

func TestSamplingHandler_FullRateKeepsAll(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	h := NewSamplingHandler(inner, SamplingConfig{
		DebugRate: 1.0,
		InfoRate:  1.0,
		WarnRate:  1.0,
		ErrorRate: 1.0,
	})
	logger := slog.New(h)

	for i := 0; i < 100; i++ {
		logger.Debug("kept")
	}

	stats := h.Stats()
	if stats.Sampled != 100 {
		t.Errorf("100%% rate should keep all: got %d", stats.Sampled)
	}
}

func TestSamplingHandler_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	h := NewSamplingHandler(inner, SamplingConfig{ErrorRate: 1.0, WarnRate: 1.0})
	h2 := h.WithAttrs([]slog.Attr{slog.String("service", "test")})

	// Should return a SamplingHandler.
	sh, ok := h2.(*SamplingHandler)
	if !ok {
		t.Fatal("WithAttrs should return *SamplingHandler")
	}
	if sh.config.ErrorRate != 1.0 {
		t.Error("config should be preserved")
	}
}

func TestSamplingHandler_WithGroup(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	h := NewSamplingHandler(inner, DefaultSamplingConfig())
	h2 := h.WithGroup("grp")

	_, ok := h2.(*SamplingHandler)
	if !ok {
		t.Fatal("WithGroup should return *SamplingHandler")
	}
}

func TestSamplingHandler_Enabled(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})

	h := NewSamplingHandler(inner, DefaultSamplingConfig())

	if h.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("debug should not be enabled with warn level inner handler")
	}
	if !h.Enabled(context.Background(), slog.LevelError) {
		t.Error("error should be enabled")
	}
}

func TestDefaultSamplingConfig(t *testing.T) {
	cfg := DefaultSamplingConfig()
	if cfg.DebugRate != 0.01 {
		t.Errorf("debug rate: got %f", cfg.DebugRate)
	}
	if cfg.InfoRate != 0.1 {
		t.Errorf("info rate: got %f", cfg.InfoRate)
	}
	if cfg.WarnRate != 1.0 {
		t.Errorf("warn rate: got %f", cfg.WarnRate)
	}
	if cfg.ErrorRate != 1.0 {
		t.Errorf("error rate: got %f", cfg.ErrorRate)
	}
}

func TestSamplingStats_EffectiveRate(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	h := NewSamplingHandler(inner, SamplingConfig{
		InfoRate:  1.0,
		ErrorRate: 1.0,
		WarnRate:  1.0,
	})

	logger := slog.New(h)
	for i := 0; i < 10; i++ {
		logger.Info("test")
	}

	stats := h.Stats()
	if stats.Rate != 1.0 {
		t.Errorf("effective rate: got %f, want 1.0", stats.Rate)
	}
}
