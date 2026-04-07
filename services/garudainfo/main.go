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

	"github.com/garudapass/gpass/services/garudainfo/config"
	"github.com/garudapass/gpass/services/garudainfo/handler"
	"github.com/garudapass/gpass/services/garudainfo/httpx"
	"github.com/garudapass/gpass/services/garudainfo/store"
)

// staticUserDataProvider is a placeholder that returns empty data.
// In production this will be replaced with a real provider that calls the Identity service.
type staticUserDataProvider struct{}

func (p *staticUserDataProvider) GetUserFields(userID string) map[string]handler.FieldValue {
	return map[string]handler.FieldValue{}
}

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

	consentStore, db, err := store.NewConsentStoreFromEnv()
	if err != nil {
		slog.Error("failed to initialize consent store", "error", err)
		os.Exit(1)
	}
	if db != nil {
		defer db.Close()
		slog.Info("consent store: postgres-backed (12factor compliant, UU PDP No. 27/2022)")
	} else {
		slog.Warn("consent store: in-memory (DEV ONLY — NOT 12factor compliant)")
	}
	consentHandler := handler.NewConsentHandler(consentStore)
	personHandler := handler.NewPersonHandler(consentStore, &staticUserDataProvider{})

	mux := http.NewServeMux()

	// Health checks
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok","service":"garudainfo"}`)
	})
	mux.HandleFunc("GET /readyz", store.ReadinessHandler(db, "garudainfo"))

	// Consent endpoints
	mux.HandleFunc("POST /api/v1/garudainfo/consents", consentHandler.Grant)
	mux.HandleFunc("GET /api/v1/garudainfo/consents", consentHandler.List)
	mux.HandleFunc("DELETE /api/v1/garudainfo/consents/{id}", consentHandler.Revoke)

	// Person data endpoint
	mux.HandleFunc("GET /api/v1/garudainfo/person", personHandler.GetPerson)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpx.RequestID(httpx.Recover(mux)),
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("garudainfo listening", "addr", server.Addr, "port", cfg.Port)
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

	slog.Info("garudainfo shut down gracefully")
}
