package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Server wraps an http.Server with graceful shutdown.
type Server struct {
	srv *http.Server
}

// Option configures the Server.
type Option func(*Server)

// WithReadTimeout sets the server read timeout.
func WithReadTimeout(d time.Duration) Option {
	return func(s *Server) {
		s.srv.ReadTimeout = d
	}
}

// WithWriteTimeout sets the server write timeout.
func WithWriteTimeout(d time.Duration) Option {
	return func(s *Server) {
		s.srv.WriteTimeout = d
	}
}

// New creates a new Server with sensible defaults.
func New(port string, handler http.Handler, opts ...Option) *Server {
	s := &Server{
		srv: &http.Server{
			Addr:              ":" + port,
			Handler:           handler,
			ReadTimeout:       15 * time.Second,
			ReadHeaderTimeout: 5 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Run starts the server and blocks until SIGINT or SIGTERM is received,
// then gracefully shuts down with a 10-second timeout.
func (s *Server) Run() error {
	errCh := make(chan error, 1)
	go func() {
		slog.Info("server starting", slog.String("addr", s.srv.Addr))
		if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		slog.Info("shutting down", slog.String("signal", sig.String()))
	case err := <-errCh:
		if err != nil {
			return err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.srv.Shutdown(ctx); err != nil {
		return err
	}

	slog.Info("server stopped gracefully")
	return nil
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}
