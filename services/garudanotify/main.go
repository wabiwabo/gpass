package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/garudapass/gpass/services/garudanotify/channel"
	"github.com/garudapass/gpass/services/garudanotify/config"
	"github.com/garudapass/gpass/services/garudanotify/handler"
	"github.com/garudapass/gpass/packages/golib/httpx"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	slog.Info("starting GarudaNotify service",
		"port", cfg.Port,
		"sms_provider", cfg.SMSProvider,
	)

	// For MVP, use mock senders. Replace with real implementations when SMTP/SMS is configured.
	emailSender := channel.NewMockEmailSender()
	smsSender := channel.NewMockSMSSender()

	notifyHandler := handler.NewNotifyHandler(emailSender, smsSender)

	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok","service":"garudanotify"}`)
	})

	readiness := httpx.NewReadiness("garudanotify", nil)
	mux.HandleFunc("GET /readyz", readiness.Handler())

	// Prometheus-format metrics for SLO/alerting
	metrics := httpx.NewMetrics("garudanotify")
	mux.HandleFunc("GET /metrics", metrics.Handler(nil))
	mux.HandleFunc("GET /version", httpx.VersionHandler(httpx.VersionInfo{Service: "garudanotify"}))

	// Notification routes
	mux.HandleFunc("POST /api/v1/notify/otp", notifyHandler.SendOTP)
	mux.HandleFunc("POST /api/v1/notify/alert", notifyHandler.SendAlert)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpx.Compress(httpx.SecurityHeaders(httpx.RequestID(httpx.AccessLog(metrics.Instrument(httpx.Recover(httpx.MaxBodyBytes(httpx.Timeout(mux, httpx.DefaultRequestTimeout), httpx.DefaultMaxBodyBytes))))), httpx.SecurityHeaderOptions{HSTS: false})),
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("GarudaNotify service listening", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	slog.Info("shutdown signal received", "signal", sig.String())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	readiness.Drain()
	slog.Info("draining: /readyz now returns 503", "drain_seconds", 10)
	time.Sleep(10 * time.Second)

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}

	slog.Info("GarudaNotify service shut down gracefully")
}
