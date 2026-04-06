// Package graceful provides a production-ready HTTP server with
// signal-based graceful shutdown, health state management,
// and configurable drain period.
package graceful

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

// State represents server health state.
type State int32

const (
	StateStarting State = iota
	StateReady
	StateDraining
	StateStopped
)

// String returns the state name.
func (s State) String() string {
	switch s {
	case StateStarting:
		return "starting"
	case StateReady:
		return "ready"
	case StateDraining:
		return "draining"
	case StateStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// Config controls graceful server behavior.
type Config struct {
	// Addr is the listen address (default ":8080").
	Addr string
	// Handler is the HTTP handler.
	Handler http.Handler
	// ShutdownTimeout is the max time to wait for in-flight requests.
	ShutdownTimeout time.Duration
	// DrainDelay is the time to wait after receiving signal before
	// starting shutdown (allows load balancers to remove the instance).
	DrainDelay time.Duration
	// Logger for lifecycle events.
	Logger *slog.Logger
	// ReadTimeout for the HTTP server.
	ReadTimeout time.Duration
	// WriteTimeout for the HTTP server.
	WriteTimeout time.Duration
	// IdleTimeout for the HTTP server.
	IdleTimeout time.Duration
	// OnReady is called when the server is ready to accept requests.
	OnReady func()
	// OnShutdown is called when shutdown begins.
	OnShutdown func()
}

// Server wraps http.Server with graceful shutdown.
type Server struct {
	cfg    Config
	state  atomic.Int32
	logger *slog.Logger
	srv    *http.Server
}

// New creates a graceful server.
func New(cfg Config) *Server {
	if cfg.Addr == "" {
		cfg.Addr = ":8080"
	}
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = 30 * time.Second
	}
	if cfg.DrainDelay < 0 {
		cfg.DrainDelay = 0
	}
	if cfg.ReadTimeout <= 0 {
		cfg.ReadTimeout = 15 * time.Second
	}
	if cfg.WriteTimeout <= 0 {
		cfg.WriteTimeout = 15 * time.Second
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = 60 * time.Second
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	s := &Server{
		cfg:    cfg,
		logger: logger,
	}
	s.state.Store(int32(StateStarting))

	s.srv = &http.Server{
		Addr:         cfg.Addr,
		Handler:      cfg.Handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	return s
}

// State returns the current server state.
func (s *Server) State() State {
	return State(s.state.Load())
}

// ListenAndServe starts the server and blocks until shutdown completes.
// It listens for SIGINT and SIGTERM to initiate graceful shutdown.
func (s *Server) ListenAndServe() error {
	return s.listenAndServe(signal.NotifyContext)
}

// listenAndServe is the internal implementation, injectable for testing.
func (s *Server) listenAndServe(
	notifyCtx func(parent context.Context, signals ...os.Signal) (context.Context, context.CancelFunc),
) error {
	ctx, stop := notifyCtx(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		s.state.Store(int32(StateReady))
		s.logger.Info("server started",
			slog.String("addr", s.cfg.Addr),
		)
		if s.cfg.OnReady != nil {
			s.cfg.OnReady()
		}
		if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		s.state.Store(int32(StateStopped))
		return err
	case <-ctx.Done():
	}

	return s.shutdown()
}

// Shutdown initiates graceful shutdown.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

func (s *Server) shutdown() error {
	s.state.Store(int32(StateDraining))
	s.logger.Info("shutdown initiated, draining connections")

	if s.cfg.OnShutdown != nil {
		s.cfg.OnShutdown()
	}

	if s.cfg.DrainDelay > 0 {
		s.logger.Info("drain delay", slog.Duration("delay", s.cfg.DrainDelay))
		time.Sleep(s.cfg.DrainDelay)
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
	defer cancel()

	err := s.srv.Shutdown(ctx)
	s.state.Store(int32(StateStopped))

	if err != nil {
		s.logger.Error("shutdown error", slog.String("error", err.Error()))
		return err
	}

	s.logger.Info("server stopped gracefully")
	return nil
}

// IsReady returns true if the server is accepting requests.
func (s *Server) IsReady() bool {
	return s.State() == StateReady
}

// HealthHandler returns an HTTP handler reporting server health state.
func (s *Server) HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state := s.State()
		if state == StateReady {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(state.String()))
	}
}
