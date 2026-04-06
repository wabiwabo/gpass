package bootstrap

import (
	"log/slog"
	"net/http"
	"sync"

	"github.com/garudapass/gpass/packages/golib/health"
	"github.com/garudapass/gpass/packages/golib/logging"
	"github.com/garudapass/gpass/packages/golib/metrics"
	"github.com/garudapass/gpass/packages/golib/middleware"
	"github.com/garudapass/gpass/packages/golib/server"
)

// ServiceConfig holds all configuration for bootstrapping a service.
type ServiceConfig struct {
	Name          string
	Port          string
	Version       string
	Environment   string // dev, staging, production
	LogLevel      string
	LogFormat     string
	EnableMetrics bool
	EnableHealth  bool
	CORSOrigins   []string
}

// route describes a registered route for documentation/debugging.
type route struct {
	Method      string
	Path        string
	Description string
}

// Service is a fully bootstrapped service with standard enterprise middleware.
type Service struct {
	Config  ServiceConfig
	Mux     *http.ServeMux
	Metrics *metrics.HTTPMetrics

	mu     sync.Mutex
	routes []route
	logCleanup func()
}

// New creates a service with standard middleware already applied.
// It sets up logging, metrics, and health endpoints based on config.
func New(cfg ServiceConfig) *Service {
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	if cfg.Environment == "" {
		cfg.Environment = "dev"
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	if cfg.LogFormat == "" {
		cfg.LogFormat = "text"
	}

	// Set up structured logging.
	cleanup := logging.Setup(logging.Config{
		ServiceName: cfg.Name,
		Environment: cfg.Environment,
		Level:       cfg.LogLevel,
		Format:      cfg.LogFormat,
	})

	mux := http.NewServeMux()

	var m *metrics.HTTPMetrics
	if cfg.EnableMetrics {
		m = metrics.New(cfg.Name)
		mux.HandleFunc("GET /metrics", m.Handler())
	}

	if cfg.EnableHealth {
		mux.HandleFunc("GET /health", health.Handler(cfg.Name))
	}

	return &Service{
		Config:     cfg,
		Mux:        mux,
		Metrics:    m,
		logCleanup: cleanup,
	}
}

// AddRoute registers a route with optional documentation.
func (s *Service) AddRoute(method, path, description string, handler http.HandlerFunc) {
	s.mu.Lock()
	s.routes = append(s.routes, route{
		Method:      method,
		Path:        path,
		Description: description,
	})
	s.mu.Unlock()

	pattern := method + " " + path
	s.Mux.HandleFunc(pattern, handler)
}

// Handler returns the fully wrapped handler (for testing).
// Applies middleware chain: Recovery -> Correlation -> AccessLog -> SecureHeaders -> CORS -> Metrics -> handler
func (s *Service) Handler() http.Handler {
	builder := middleware.NewBuilder().
		Use(middleware.Recovery).
		Use(middleware.Correlation).
		Use(middleware.AccessLog).
		Use(middleware.SecureHeaders).
		UseIf(len(s.Config.CORSOrigins) > 0, middleware.CORS(s.Config.CORSOrigins)).
		UseIf(s.Config.Environment == "production" || s.Config.Environment == "staging",
			middleware.HSTS(63072000))

	if s.Metrics != nil {
		builder.Use(s.Metrics.Middleware)
	}

	return builder.Build(s.Mux)
}

// Run starts the service with graceful shutdown.
func (s *Service) Run() error {
	if s.logCleanup != nil {
		defer s.logCleanup()
	}

	handler := s.Handler()

	slog.Info("service starting",
		slog.String("name", s.Config.Name),
		slog.String("port", s.Config.Port),
		slog.String("version", s.Config.Version),
		slog.String("environment", s.Config.Environment),
	)

	srv := server.New(s.Config.Port, handler)
	return srv.Run()
}
