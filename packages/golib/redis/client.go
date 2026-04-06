package redis

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// Config holds Redis connection configuration.
type Config struct {
	URL          string        // redis://[:password@]host:port[/db]
	MaxRetries   int           // default 3
	DialTimeout  time.Duration // default 5s
	ReadTimeout  time.Duration // default 3s
	WriteTimeout time.Duration // default 3s
	PoolSize     int           // default 10
}

// WithDefaults returns a copy of cfg with zero-value fields filled with defaults.
func (cfg Config) WithDefaults() Config {
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = 5 * time.Second
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 3 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 3 * time.Second
	}
	if cfg.PoolSize == 0 {
		cfg.PoolSize = 10
	}
	return cfg
}

// Validate checks that the configuration is valid.
func (cfg *Config) Validate() error {
	if cfg.URL == "" {
		return fmt.Errorf("redis: URL is required")
	}

	u, err := url.Parse(cfg.URL)
	if err != nil {
		return fmt.Errorf("redis: invalid URL: %w", err)
	}

	if u.Scheme != "redis" && u.Scheme != "rediss" {
		return fmt.Errorf("redis: URL scheme must be redis:// or rediss://, got %q", u.Scheme)
	}

	return nil
}

// Client wraps Redis operations needed by GarudaPass services.
type Client struct {
	url string
	cfg Config
}

// New creates a new Redis client.
func New(cfg Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	cfg = cfg.WithDefaults()

	return &Client{
		url: cfg.URL,
		cfg: cfg,
	}, nil
}

// Health checks the Redis connection.
func (c *Client) Health(ctx context.Context) error {
	// In a real implementation, this would ping the Redis server.
	// The actual Redis driver (e.g., go-redis) would be used here.
	return nil
}

// Close closes the Redis connection.
func (c *Client) Close() error {
	// In a real implementation, this would close the underlying connection.
	return nil
}
