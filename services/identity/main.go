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

	"github.com/garudapass/gpass/services/identity/config"
	"github.com/garudapass/gpass/services/identity/dukcapil"
	"github.com/garudapass/gpass/services/identity/handler"
	"github.com/garudapass/gpass/services/identity/otp"
	"github.com/garudapass/gpass/packages/golib/httpx"
	"github.com/garudapass/gpass/services/identity/store"
	"github.com/redis/go-redis/v9"
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

	slog.Info("starting Identity service",
		"port", cfg.Port,
		"dukcapil_mode", cfg.DukcapilMode,
	)

	// Redis for OTP
	redisOpt, err := redis.ParseURL(cfg.OTPRedisURL)
	if err != nil {
		slog.Error("invalid OTP redis URL", "error", err)
		os.Exit(1)
	}
	rdb := redis.NewClient(redisOpt)
	defer rdb.Close()

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		slog.Warn("redis not reachable, OTP will fail until redis is available", "error", err)
	} else {
		slog.Info("redis connected")
	}

	// Dukcapil client
	dukcapilClient := dukcapil.NewClient(cfg.DukcapilURL, cfg.DukcapilAPIKey, cfg.DukcapilTimeout)

	// OTP service
	otpService := otp.NewService(rdb)

	// Registration handler
	registerHandler := handler.NewRegisterHandler(handler.RegisterDeps{
		Dukcapil: dukcapilClient,
		OTP:      otpService,
		NIKKey:   cfg.ServerNIKKey,
	})

	// Deletion handler (UU PDP compliance)
	deletionStore, delDB, err := store.NewDeletionStoreFromEnv()
	if err != nil {
		slog.Error("failed to initialize deletion store", "error", err)
		os.Exit(1)
	}
	if delDB != nil {
		defer delDB.Close()
		slog.Info("deletion store: postgres-backed (12factor compliant, UU PDP No. 27/2022)")
	} else {
		slog.Warn("deletion store: in-memory (DEV ONLY — NOT 12factor compliant)")
	}
	auditEmitter := &logAuditEmitter{}
	deletionHandler := handler.NewDeletionHandler(deletionStore, auditEmitter)

	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("POST /api/v1/register/initiate", registerHandler.Initiate)
	mux.HandleFunc("POST /api/v1/identity/deletion", deletionHandler.RequestDeletion)
	mux.HandleFunc("GET /api/v1/identity/deletion/{id}", deletionHandler.GetDeletionStatus)

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok","service":"identity"}`)
	})
	mux.HandleFunc("GET /readyz", store.ReadinessHandler(delDB, "identity"))

	// Prometheus-format metrics for SLO/alerting
	metrics := httpx.NewMetrics("identity")
	mux.HandleFunc("GET /metrics", metrics.Handler(delDB))

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpx.RequestID(httpx.AccessLog(metrics.Instrument(httpx.Recover(httpx.MaxBodyBytes(mux, 25*1024*1024))))),
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    10 << 20, // 10MB for selfie uploads
	}

	go func() {
		slog.Info("Identity service listening", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	slog.Info("shutdown signal received", "signal", sig.String())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}

	slog.Info("Identity service shut down gracefully")
}

// logAuditEmitter implements handler.AuditEmitter by writing structured log entries.
// In production, this would publish to Kafka for the audit service.
type logAuditEmitter struct{}

func (e *logAuditEmitter) Emit(eventType, userID, resourceID string, metadata map[string]string) error {
	slog.Info("audit event",
		"event_type", eventType,
		"user_id", userID,
		"resource_id", resourceID,
		"metadata", metadata,
	)
	return nil
}
