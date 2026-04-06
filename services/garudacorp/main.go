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

	"github.com/garudapass/gpass/services/garudacorp/ahu"
	"github.com/garudapass/gpass/services/garudacorp/config"
	"github.com/garudapass/gpass/services/garudacorp/handler"
	"github.com/garudapass/gpass/services/garudacorp/oss"
	"github.com/garudapass/gpass/services/garudacorp/store"
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

	slog.Info("starting GarudaCorp service",
		"port", cfg.Port,
	)

	// Stores
	entityStore := store.NewInMemoryEntityStore()
	roleStore := store.NewInMemoryRoleStore()

	// Clients
	ahuClient := ahu.NewClient(cfg.AHUURL, cfg.AHUAPIKey, cfg.AHUTimeout)
	ossClient := oss.NewClient(cfg.OSSURL, cfg.OSSAPIKey, cfg.OSSTimeout)

	// Handlers
	registerHandler := handler.NewRegisterHandler(handler.RegisterDeps{
		AHU:         ahuClient,
		EntityStore: entityStore,
		RoleStore:   roleStore,
		NIKKey:      cfg.ServerNIKKey,
	})
	roleHandler := handler.NewRoleHandler(roleStore, entityStore)
	entityHandler := handler.NewEntityHandler(entityStore, ossClient)

	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok","service":"garudacorp"}`)
	})

	// API routes
	mux.HandleFunc("POST /api/v1/corp/register", registerHandler.Register)
	mux.HandleFunc("GET /api/v1/corp/entities/{id}", entityHandler.GetEntity)
	mux.HandleFunc("POST /api/v1/corp/entities/{entity_id}/roles", roleHandler.AssignRole)
	mux.HandleFunc("GET /api/v1/corp/entities/{entity_id}/roles", roleHandler.ListRoles)
	mux.HandleFunc("DELETE /api/v1/corp/entities/{entity_id}/roles/{role_id}", roleHandler.RevokeRole)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("GarudaCorp service listening", "addr", server.Addr)
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

	slog.Info("GarudaCorp service shut down gracefully")
}
