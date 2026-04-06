package main

import (
	"context"
	"encoding/hex"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/garudapass/gpass/apps/bff/config"
	"github.com/garudapass/gpass/apps/bff/handler"
	"github.com/garudapass/gpass/apps/bff/middleware"
	"github.com/garudapass/gpass/apps/bff/session"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Configure structured logging based on environment
	logLevel := slog.LevelInfo
	if cfg.IsProd() {
		logLevel = slog.LevelWarn
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	slog.Info("starting BFF",
		"port", cfg.Port,
		"env", string(cfg.Environment),
		"keycloak_url", cfg.KeycloakURL,
	)

	// Redis
	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		slog.Error("invalid redis URL", "error", err)
		os.Exit(1)
	}
	rdb := redis.NewClient(opt)
	defer rdb.Close()

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		slog.Warn("redis not reachable, sessions will fail until redis is available", "error", err)
	} else {
		slog.Info("redis connected")
	}

	// Session encryption key from the first 32 bytes of session secret
	var encKey []byte
	if len(cfg.SessionSecret) >= 64 {
		encKey, _ = hex.DecodeString(cfg.SessionSecret[:64])
	}
	if len(encKey) != 32 {
		// Fallback: derive 32 bytes from the secret using simple truncation
		raw := []byte(cfg.SessionSecret)
		if len(raw) >= 32 {
			encKey = raw[:32]
		} else {
			encKey = nil // No encryption, not recommended
			slog.Warn("session encryption disabled: session secret too short for AES-256")
		}
	}

	store, err := session.NewRedisStore(rdb, encKey)
	if err != nil {
		slog.Error("failed to create session store", "error", err)
		os.Exit(1)
	}

	authHandler := handler.NewAuthHandler(handler.AuthConfig{
		IssuerURL:    cfg.IssuerURL(),
		ClientID:     cfg.ClientID,
		RedirectURI:  cfg.RedirectURI,
		FrontendURL:  cfg.FrontendURL,
		CookieDomain: cfg.CookieDomain,
		SecureCookie: cfg.IsSecure(),
	}, store)
	sessionHandler := handler.NewSessionHandler(store)

	mux := http.NewServeMux()

	// Auth routes (no CSRF — these initiate/complete OAuth flows)
	mux.HandleFunc("GET /auth/login", authHandler.Login)
	mux.HandleFunc("GET /auth/callback", authHandler.Callback)
	mux.HandleFunc("POST /auth/logout", authHandler.Logout)
	mux.HandleFunc("GET /auth/session", sessionHandler.GetSession)

	// Health check (no middleware — used by load balancers and K8s probes)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Readiness check — verifies Redis connectivity
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := rdb.Ping(r.Context()).Err(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"not_ready","reason":"redis_unavailable"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready"}`))
	})

	// API routes (with CSRF protection)
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("GET /api/v1/me", sessionHandler.GetSession)
	mux.Handle("/api/", middleware.CSRF(apiMux))

	// Enterprise middleware chain (outermost first):
	// Recovery -> RequestID -> AccessLog -> SecurityHeaders -> Handler
	var rootHandler http.Handler = mux
	rootHandler = middleware.SecurityHeaders(rootHandler)
	rootHandler = middleware.AccessLog(rootHandler)
	rootHandler = middleware.RequestID(rootHandler)
	rootHandler = middleware.Recovery(rootHandler)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           rootHandler,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1MB
	}

	go func() {
		slog.Info("BFF listening", "addr", server.Addr)
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

	slog.Info("BFF shut down gracefully")
}
