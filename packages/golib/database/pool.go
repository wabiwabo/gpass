package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/lib/pq"
)

// Config holds database connection configuration.
type Config struct {
	URL             string
	MaxOpenConns    int           // default 25
	MaxIdleConns    int           // default 5
	ConnMaxLifetime time.Duration // default 5m
	ConnMaxIdleTime time.Duration // default 1m
}

// WithDefaults returns a copy of cfg with zero-value fields filled with defaults.
func (cfg Config) WithDefaults() Config {
	if cfg.MaxOpenConns == 0 {
		cfg.MaxOpenConns = 25
	}
	if cfg.MaxIdleConns == 0 {
		cfg.MaxIdleConns = 5
	}
	if cfg.ConnMaxLifetime == 0 {
		cfg.ConnMaxLifetime = 5 * time.Minute
	}
	if cfg.ConnMaxIdleTime == 0 {
		cfg.ConnMaxIdleTime = 1 * time.Minute
	}
	return cfg
}

// Pool wraps *sql.DB with health checking and structured logging.
type Pool struct {
	DB  *sql.DB
	cfg Config
}

// New creates a new database connection pool.
func New(cfg Config) (*Pool, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("database: URL is required")
	}

	cfg = cfg.WithDefaults()

	db, err := sql.Open("postgres", cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("database: failed to open connection: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	slog.Info("database pool created",
		"max_open_conns", cfg.MaxOpenConns,
		"max_idle_conns", cfg.MaxIdleConns,
		"conn_max_lifetime", cfg.ConnMaxLifetime,
		"conn_max_idle_time", cfg.ConnMaxIdleTime,
	)

	return &Pool{DB: db, cfg: cfg}, nil
}

// Health checks the database connection.
func (p *Pool) Health(ctx context.Context) error {
	if err := p.DB.PingContext(ctx); err != nil {
		return fmt.Errorf("database: health check failed: %w", err)
	}
	return nil
}

// Close closes the connection pool.
func (p *Pool) Close() error {
	slog.Info("closing database pool")
	return p.DB.Close()
}
