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

	"github.com/garudapass/gpass/services/garudasign/audit"
	"github.com/garudapass/gpass/services/garudasign/config"
	"github.com/garudapass/gpass/services/garudasign/handler"
	"github.com/garudapass/gpass/services/garudasign/signing"
	"github.com/garudapass/gpass/services/garudasign/storage"
	"github.com/garudapass/gpass/services/garudasign/store"
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

	// Create stores
	stores, err := store.NewStoresFromEnv()
	if err != nil {
		slog.Error("failed to initialize stores", "error", err)
		os.Exit(1)
	}
	if stores.DB != nil {
		defer stores.DB.Close()
		slog.Info("stores: postgres-backed (12factor compliant, ETSI EN 319 142)")
	} else {
		slog.Warn("stores: in-memory (DEV ONLY — NOT 12factor compliant)")
	}
	certStore := stores.Certificate
	reqStore := stores.Request
	docStore := stores.Document

	// Create file storage
	fileStorage := storage.NewFileStorage(cfg.DocumentStoragePath)

	// Create signing client
	var signingURL string
	if cfg.IsSimulator() {
		signingURL = cfg.SigningSimURL
	} else {
		signingURL = cfg.EJBCAURL
	}
	signClient := signing.NewClient(signingURL, 30*time.Second)

	// Create audit emitter
	auditEmitter := audit.NewLogEmitter()

	// Create handlers
	certHandler := handler.NewCertificateHandler(handler.CertificateDeps{
		CertStore:    certStore,
		SignClient:   signClient,
		AuditEmitter: auditEmitter,
		ValidityDays: cfg.CertValidityDays,
	})

	docHandler := handler.NewDocumentHandler(handler.DocumentDeps{
		CertStore:    certStore,
		RequestStore: reqStore,
		DocStore:     docStore,
		FileStorage:  fileStorage,
		SignClient:   signClient,
		AuditEmitter: auditEmitter,
		MaxSizeMB:    cfg.MaxSizeMB,
		RequestTTL:   cfg.RequestTTL,
	})

	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok","service":"garudasign"}`)
	})
	mux.HandleFunc("GET /readyz", store.ReadinessHandler(stores.DB, "garudasign"))

	// Certificate routes
	mux.HandleFunc("POST /api/v1/sign/certificates/request", certHandler.RequestCertificate)
	mux.HandleFunc("GET /api/v1/sign/certificates", certHandler.ListCertificates)

	// Document routes
	mux.HandleFunc("POST /api/v1/sign/documents", docHandler.Upload)
	mux.HandleFunc("POST /api/v1/sign/documents/{id}/sign", docHandler.Sign)
	mux.HandleFunc("GET /api/v1/sign/documents/{id}", docHandler.GetStatus)
	mux.HandleFunc("GET /api/v1/sign/documents/{id}/download", docHandler.Download)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("garudasign service listening",
			"addr", server.Addr,
			"signing_mode", cfg.SigningMode,
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

	slog.Info("garudasign service shut down gracefully")
}
