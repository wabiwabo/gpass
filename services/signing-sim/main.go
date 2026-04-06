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

	"github.com/garudapass/gpass/services/signing-sim/ca"
	"github.com/garudapass/gpass/services/signing-sim/handler"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	port := os.Getenv("SIGNING_SIM_PORT")
	if port == "" {
		port = "4008"
	}

	// Initialize CA
	rootCA, err := ca.NewCA()
	if err != nil {
		slog.Error("failed to initialize CA", "error", err)
		os.Exit(1)
	}

	slog.Info("signing-sim CA initialized",
		"root_cn", rootCA.RootCN(),
		"root_serial", rootCA.RootSerial(),
	)

	// Handlers
	certHandler := handler.NewCertificateHandler(rootCA)
	signHandler := handler.NewSignHandler(rootCA)

	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok","service":"signing-sim"}`)
	})

	// API routes
	mux.HandleFunc("POST /certificates/issue", certHandler.Issue)
	mux.HandleFunc("POST /sign/pades", signHandler.Sign)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("signing-sim service listening", "addr", server.Addr)
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

	slog.Info("signing-sim service shut down gracefully")
}
