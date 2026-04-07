package store

import (
	"database/sql"
	"testing"
	"time"
)

// TestNewStoresFromEnv_NoDSNFallsBackToInMemory pins the in-memory
// fallback path when neither GARUDACORP_DB_URL nor DATABASE_URL is set.
// This is the development/testing default.
func TestNewStoresFromEnv_NoDSNFallsBackToInMemory(t *testing.T) {
	t.Setenv("GARUDACORP_DB_URL", "")
	t.Setenv("DATABASE_URL", "")
	s, err := NewStoresFromEnv()
	if err != nil {
		t.Fatalf("NewStoresFromEnv: %v", err)
	}
	if s.Entity == nil || s.Role == nil || s.UBO == nil {
		t.Errorf("nil store: %+v", s)
	}
	if s.DB != nil {
		t.Error("DB should be nil for in-memory mode")
	}
}

// TestNewStoresFromEnv_BadDSNRejected pins the sql.Open / Ping error
// branch. We use a syntactically valid but unreachable DSN so Ping fails
// fast without needing a real Postgres.
func TestNewStoresFromEnv_BadDSNRejected(t *testing.T) {
	t.Setenv("GARUDACORP_DB_URL", "postgres://user:pass@127.0.0.1:1/db?sslmode=disable&connect_timeout=1")
	_, err := NewStoresFromEnv()
	if err == nil {
		t.Error("expected ping error for unreachable DSN")
	}
}

// TestNewStoresFromEnv_DataBaseURLFallback pins that DATABASE_URL is
// honored when GARUDACORP_DB_URL is unset.
func TestNewStoresFromEnv_DataBaseURLFallback(t *testing.T) {
	t.Setenv("GARUDACORP_DB_URL", "")
	t.Setenv("DATABASE_URL", "postgres://user:pass@127.0.0.1:1/db?sslmode=disable&connect_timeout=1")
	_, err := NewStoresFromEnv()
	if err == nil {
		t.Error("DATABASE_URL fallback should still attempt connection")
	}
}

// TestEnvInt_DefaultsAndOverrides pins the envInt parser including the
// "non-numeric → default" branch and the "negative → default" branch.
func TestEnvInt_DefaultsAndOverrides(t *testing.T) {
	t.Setenv("DB_FOO", "")
	if got := envInt("DB_FOO", 25); got != 25 {
		t.Errorf("unset: %d", got)
	}
	t.Setenv("DB_FOO", "50")
	if got := envInt("DB_FOO", 25); got != 50 {
		t.Errorf("override: %d", got)
	}
	t.Setenv("DB_FOO", "not-a-number")
	if got := envInt("DB_FOO", 25); got != 25 {
		t.Errorf("bad: %d", got)
	}
	t.Setenv("DB_FOO", "-5")
	if got := envInt("DB_FOO", 25); got != 25 {
		t.Errorf("negative: %d", got)
	}
}

// TestEnvDuration_DefaultsAndOverrides pins the envDuration parser
// including bad/negative branches.
func TestEnvDuration_DefaultsAndOverrides(t *testing.T) {
	t.Setenv("DB_BAR", "")
	if got := envDuration("DB_BAR", 30*time.Minute); got != 30*time.Minute {
		t.Errorf("unset: %v", got)
	}
	t.Setenv("DB_BAR", "60")
	if got := envDuration("DB_BAR", 30*time.Minute); got != 60*time.Second {
		t.Errorf("override: %v", got)
	}
	t.Setenv("DB_BAR", "abc")
	if got := envDuration("DB_BAR", 30*time.Minute); got != 30*time.Minute {
		t.Errorf("bad: %v", got)
	}
	t.Setenv("DB_BAR", "-1")
	if got := envDuration("DB_BAR", 30*time.Minute); got != 30*time.Minute {
		t.Errorf("negative: %v", got)
	}
}

// TestConfigurePool_AppliesDefaults pins the configurePool sets-from-env
// path. We use a real *sql.DB opened against an in-memory sqlite-style
// driver registered in the package's existing fake driver pool — but
// since garudacorp doesn't have one, just verify configurePool does
// not panic on a *sql.DB created via the postgres driver name (no Open
// performed). We use a closed *sql.DB stub.
func TestConfigurePool_AppliesEnvOverrides(t *testing.T) {
	// Open against the registered postgres driver — this returns a *sql.DB
	// without actually dialing anything. configurePool only calls SetMax*
	// methods which don't touch the network.
	db, err := sql.Open("postgres", "postgres://localhost:1/x")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	t.Setenv("DB_MAX_OPEN_CONNS", "75")
	t.Setenv("DB_MAX_IDLE_CONNS", "30")
	t.Setenv("DB_CONN_MAX_LIFETIME_SEC", "1800")
	t.Setenv("DB_CONN_MAX_IDLE_TIME_SEC", "300")

	configurePool(db)
	stats := db.Stats()
	if stats.MaxOpenConnections != 75 {
		t.Errorf("MaxOpenConnections = %d, want 75", stats.MaxOpenConnections)
	}
}
