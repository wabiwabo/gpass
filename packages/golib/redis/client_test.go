package redis

import (
	"testing"
	"time"
)

func TestConfigWithDefaults(t *testing.T) {
	cfg := Config{}.WithDefaults()

	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}
	if cfg.DialTimeout != 5*time.Second {
		t.Errorf("DialTimeout = %v, want 5s", cfg.DialTimeout)
	}
	if cfg.ReadTimeout != 3*time.Second {
		t.Errorf("ReadTimeout = %v, want 3s", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 3*time.Second {
		t.Errorf("WriteTimeout = %v, want 3s", cfg.WriteTimeout)
	}
	if cfg.PoolSize != 10 {
		t.Errorf("PoolSize = %d, want 10", cfg.PoolSize)
	}
}

func TestConfigWithDefaultsPreservesExplicitValues(t *testing.T) {
	cfg := Config{
		MaxRetries:   5,
		DialTimeout:  10 * time.Second,
		ReadTimeout:  7 * time.Second,
		WriteTimeout: 7 * time.Second,
		PoolSize:     20,
	}.WithDefaults()

	if cfg.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", cfg.MaxRetries)
	}
	if cfg.DialTimeout != 10*time.Second {
		t.Errorf("DialTimeout = %v, want 10s", cfg.DialTimeout)
	}
	if cfg.PoolSize != 20 {
		t.Errorf("PoolSize = %d, want 20", cfg.PoolSize)
	}
}

func TestConfigValidateRejectsEmptyURL(t *testing.T) {
	cfg := &Config{}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for empty URL, got nil")
	}
	if got := err.Error(); got != "redis: URL is required" {
		t.Errorf("error = %q, want %q", got, "redis: URL is required")
	}
}

func TestConfigValidateRejectsNonRedisScheme(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"http scheme", "http://localhost:6379"},
		{"tcp scheme", "tcp://localhost:6379"},
		{"no scheme", "localhost:6379"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{URL: tt.url}
			err := cfg.Validate()
			if err == nil {
				t.Fatalf("expected error for URL %q, got nil", tt.url)
			}
		})
	}
}

func TestConfigValidateAcceptsRedisSchemes(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"redis", "redis://localhost:6379"},
		{"rediss", "rediss://localhost:6379"},
		{"redis with auth", "redis://:password@localhost:6379/0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{URL: tt.url}
			if err := cfg.Validate(); err != nil {
				t.Errorf("unexpected error for URL %q: %v", tt.url, err)
			}
		})
	}
}

func TestNewReturnsErrorForEmptyURL(t *testing.T) {
	_, err := New(Config{})
	if err == nil {
		t.Fatal("expected error for empty URL, got nil")
	}
}

func TestNewReturnsErrorForInvalidScheme(t *testing.T) {
	_, err := New(Config{URL: "http://localhost:6379"})
	if err == nil {
		t.Fatal("expected error for http URL, got nil")
	}
}

func TestNewSucceedsForValidConfig(t *testing.T) {
	client, err := New(Config{URL: "redis://localhost:6379"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("client is nil")
	}
	if client.url != "redis://localhost:6379" {
		t.Errorf("url = %q, want %q", client.url, "redis://localhost:6379")
	}
}
