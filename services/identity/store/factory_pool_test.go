package store

import (
	"database/sql"
	"testing"
	"time"
)

// TestNewDeletionStoreFromEnv_NoDSNFallsBackToInMemory pins the in-memory
// fallback path.
func TestNewDeletionStoreFromEnv_NoDSNFallsBackToInMemory(t *testing.T) {
	t.Setenv("IDENTITY_DB_URL", "")
	t.Setenv("DATABASE_URL", "")
	s, db, err := NewDeletionStoreFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if s == nil {
		t.Fatal("nil store")
	}
	if db != nil {
		t.Error("DB should be nil for in-memory mode")
	}
}

// TestNewDeletionStoreFromEnv_BadDSNRejected pins the Ping error branch.
func TestNewDeletionStoreFromEnv_BadDSNRejected(t *testing.T) {
	t.Setenv("IDENTITY_DB_URL", "postgres://user:pass@127.0.0.1:1/db?sslmode=disable&connect_timeout=1")
	if _, _, err := NewDeletionStoreFromEnv(); err == nil {
		t.Error("expected ping error")
	}
}

// TestNewDeletionStoreFromEnv_DataBaseURLFallback pins the secondary
// env var path.
func TestNewDeletionStoreFromEnv_DataBaseURLFallback(t *testing.T) {
	t.Setenv("IDENTITY_DB_URL", "")
	t.Setenv("DATABASE_URL", "postgres://user:pass@127.0.0.1:1/db?sslmode=disable&connect_timeout=1")
	if _, _, err := NewDeletionStoreFromEnv(); err == nil {
		t.Error("expected ping error via DATABASE_URL")
	}
}

// TestEnvInt_Branches pins envInt across unset/override/bad/negative.
func TestEnvInt_Branches(t *testing.T) {
	t.Setenv("DB_X", "")
	if envInt("DB_X", 25) != 25 {
		t.Error("unset")
	}
	t.Setenv("DB_X", "50")
	if envInt("DB_X", 25) != 50 {
		t.Error("override")
	}
	t.Setenv("DB_X", "abc")
	if envInt("DB_X", 25) != 25 {
		t.Error("bad")
	}
	t.Setenv("DB_X", "-5")
	if envInt("DB_X", 25) != 25 {
		t.Error("negative")
	}
}

// TestEnvDuration_Branches pins envDuration across unset/override/bad/neg.
func TestEnvDuration_Branches(t *testing.T) {
	t.Setenv("DB_Y", "")
	if envDuration("DB_Y", 30*time.Minute) != 30*time.Minute {
		t.Error("unset")
	}
	t.Setenv("DB_Y", "60")
	if envDuration("DB_Y", 30*time.Minute) != 60*time.Second {
		t.Error("override")
	}
	t.Setenv("DB_Y", "abc")
	if envDuration("DB_Y", 30*time.Minute) != 30*time.Minute {
		t.Error("bad")
	}
}

// TestConfigurePool_AppliesEnvOverrides pins configurePool's env-driven
// SetMax* calls without needing a real database connection.
func TestConfigurePool_AppliesEnvOverrides(t *testing.T) {
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
	if db.Stats().MaxOpenConnections != 75 {
		t.Errorf("MaxOpenConnections = %d", db.Stats().MaxOpenConnections)
	}
}
