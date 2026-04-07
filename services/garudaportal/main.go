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

	"github.com/garudapass/gpass/services/garudaportal/config"
	"github.com/garudapass/gpass/services/garudaportal/handler"
	"github.com/garudapass/gpass/packages/golib/httpx"
	"github.com/garudapass/gpass/services/garudaportal/store"
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

	slog.Info("starting GarudaPortal service",
		"port", cfg.Port,
	)

	// Stores
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()
	webhookStore := store.NewInMemoryWebhookStore()
	usageStore := store.NewInMemoryUsageStore()

	// Handlers
	appHandler := handler.NewAppHandler(appStore)
	keyHandler := handler.NewKeyHandler(appStore, keyStore)
	webhookHandler := handler.NewWebhookHandler(appStore, webhookStore)
	usageHandler := handler.NewUsageHandler(appStore, usageStore)
	validateHandler := handler.NewValidateHandler(appStore, keyStore, usageStore)

	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok","service":"garudaportal"}`)
	})

	// Prometheus-format metrics for SLO/alerting
	metrics := httpx.NewMetrics("garudaportal")
	mux.HandleFunc("GET /metrics", metrics.Handler(nil))

	// App management
	mux.HandleFunc("POST /api/v1/portal/apps", appHandler.CreateApp)
	mux.HandleFunc("GET /api/v1/portal/apps", appHandler.ListApps)
	mux.HandleFunc("GET /api/v1/portal/apps/{id}", appHandler.GetApp)
	mux.HandleFunc("PATCH /api/v1/portal/apps/{id}", appHandler.UpdateApp)

	// Key management
	mux.HandleFunc("POST /api/v1/portal/apps/{app_id}/keys", keyHandler.CreateKey)
	mux.HandleFunc("GET /api/v1/portal/apps/{app_id}/keys", keyHandler.ListKeys)
	mux.HandleFunc("DELETE /api/v1/portal/apps/{app_id}/keys/{key_id}", keyHandler.RevokeKey)

	// Webhook management
	mux.HandleFunc("POST /api/v1/portal/apps/{app_id}/webhooks", webhookHandler.Subscribe)
	mux.HandleFunc("GET /api/v1/portal/apps/{app_id}/webhooks", webhookHandler.ListWebhooks)
	mux.HandleFunc("DELETE /api/v1/portal/apps/{app_id}/webhooks/{webhook_id}", webhookHandler.DeleteWebhook)

	// Usage stats
	mux.HandleFunc("GET /api/v1/portal/apps/{app_id}/usage", usageHandler.GetUsage)

	// Internal key validation (for Kong)
	mux.HandleFunc("POST /internal/keys/validate", validateHandler.ValidateKey)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpx.RequestID(httpx.AccessLog(metrics.Instrument(httpx.Recover(httpx.MaxBodyBytes(mux, httpx.DefaultMaxBodyBytes))))),
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("GarudaPortal service listening", "addr", server.Addr)
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

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}

	slog.Info("GarudaPortal service shut down gracefully")
}
