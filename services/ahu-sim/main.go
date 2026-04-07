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

	"github.com/garudapass/gpass/services/ahu-sim/handler"
	"github.com/garudapass/gpass/packages/golib/httpx"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	port := os.Getenv("AHUSIM_PORT")
	if port == "" {
		port = "4004"
	}

	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok","service":"ahu-sim"}`)
	})

	// Prometheus-format metrics
	metrics := httpx.NewMetrics("ahu-sim")
	mux.HandleFunc("GET /metrics", metrics.Handler(nil))

	// Company endpoints
	mux.HandleFunc("POST /api/v1/ahu/company/search", handler.SearchCompany)
	mux.HandleFunc("GET /api/v1/ahu/company/{sk}/officers", handler.GetOfficers)
	mux.HandleFunc("GET /api/v1/ahu/company/{sk}/shareholders", handler.GetShareholders)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           httpx.SecurityHeaders(httpx.RequestID(httpx.AccessLog(metrics.Instrument(httpx.Recover(httpx.MaxBodyBytes(mux, httpx.DefaultMaxBodyBytes))))), httpx.SecurityHeaderOptions{HSTS: false}),
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("ahu-sim listening", "addr", server.Addr, "port", port)
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

	slog.Info("ahu-sim shut down gracefully")
}
