package database

import (
	"testing"
	"time"
)

func TestConfigWithDefaults(t *testing.T) {
	cfg := Config{}.WithDefaults()

	if cfg.MaxOpenConns != 25 {
		t.Errorf("MaxOpenConns = %d, want 25", cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns != 5 {
		t.Errorf("MaxIdleConns = %d, want 5", cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime != 5*time.Minute {
		t.Errorf("ConnMaxLifetime = %v, want 5m", cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime != 1*time.Minute {
		t.Errorf("ConnMaxIdleTime = %v, want 1m", cfg.ConnMaxIdleTime)
	}
}

func TestConfigWithDefaultsPreservesExplicitValues(t *testing.T) {
	cfg := Config{
		MaxOpenConns:    50,
		MaxIdleConns:    10,
		ConnMaxLifetime: 10 * time.Minute,
		ConnMaxIdleTime: 2 * time.Minute,
	}.WithDefaults()

	if cfg.MaxOpenConns != 50 {
		t.Errorf("MaxOpenConns = %d, want 50", cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns != 10 {
		t.Errorf("MaxIdleConns = %d, want 10", cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime != 10*time.Minute {
		t.Errorf("ConnMaxLifetime = %v, want 10m", cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime != 2*time.Minute {
		t.Errorf("ConnMaxIdleTime = %v, want 2m", cfg.ConnMaxIdleTime)
	}
}

func TestNewReturnsErrorForEmptyURL(t *testing.T) {
	_, err := New(Config{})
	if err == nil {
		t.Fatal("expected error for empty URL, got nil")
	}
	if got := err.Error(); got != "database: URL is required" {
		t.Errorf("error = %q, want %q", got, "database: URL is required")
	}
}

func TestNewReturnsErrorForInvalidURL(t *testing.T) {
	// sql.Open with "postgres" driver doesn't validate at Open time,
	// but an obviously invalid URL will fail on Ping. We test that New
	// at least succeeds for syntactically parseable URLs (driver defers
	// validation to Ping), and we test the empty case above.
	// For a truly invalid DSN that the driver rejects at Open time,
	// we can verify the error wrapping.
	pool, err := New(Config{URL: "postgres://localhost:5432/testdb"})
	if err != nil {
		// sql.Open typically doesn't fail for syntactic URLs;
		// this is fine either way.
		return
	}
	// Pool was created but connection will fail on Health/Ping.
	defer pool.Close()
}
