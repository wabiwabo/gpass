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

	"github.com/garudapass/gpass/services/garudaaudit/config"
	"github.com/garudapass/gpass/services/garudaaudit/handler"
	"github.com/garudapass/gpass/services/garudaaudit/httpx"
	"github.com/garudapass/gpass/services/garudaaudit/store"
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

	// Create stores — Postgres if DATABASE_URL set, in-memory fallback for dev
	auditStore, db, err := store.NewFromEnv()
	if err != nil {
		slog.Error("failed to initialize audit store", "error", err)
		os.Exit(1)
	}
	if db != nil {
		defer db.Close()
		slog.Info("audit store: postgres-backed (12factor compliant, PP 71/2019 retention)")
	} else {
		slog.Warn("audit store: in-memory (DEV ONLY — data lost on restart, NOT PP 71/2019 compliant)")
	}

	// Create handlers
	auditHandler := handler.NewAuditHandler(auditStore)
	statsHandler := handler.NewStatsHandler(auditStore)

	mux := http.NewServeMux()

	// Health checks
	// /health is a liveness probe — always 200 if the process is up
	// /readyz is a readiness probe — 503 if the database is unreachable
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok","service":"garudaaudit"}`)
	})
	mux.HandleFunc("GET /readyz", store.ReadinessHandler(db, "garudaaudit"))

	// Audit event routes
	mux.HandleFunc("POST /api/v1/audit/events", auditHandler.IngestEvent)
	mux.HandleFunc("GET /api/v1/audit/events", auditHandler.QueryEvents)
	mux.HandleFunc("GET /api/v1/audit/events/{id}", auditHandler.GetEvent)

	// Stats route
	mux.HandleFunc("GET /api/v1/audit/stats", statsHandler.GetStats)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpx.RequestID(httpx.Recover(mux)),
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("garudaaudit service listening",
			"addr", server.Addr,
			"retention_days", cfg.RetentionDays,
		)
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

	slog.Info("garudaaudit service shut down gracefully")
}
