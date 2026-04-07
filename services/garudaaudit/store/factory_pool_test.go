package store

import (
	"database/sql"
	"testing"
	"time"
)

// TestNewFromEnv_NoDSNFallsBackToInMemory pins the in-memory fallback path.
func TestNewFromEnv_NoDSNFallsBackToInMemory(t *testing.T) {
	t.Setenv("GARUDAAUDIT_DB_URL", "")
	t.Setenv("DATABASE_URL", "")
	s, db, err := NewFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if s == nil {
		t.Fatal("nil store")
	}
	if db != nil {
		t.Error("DB should be nil for in-memory")
	}
}

// TestNewFromEnv_BadDSNRejected pins the Ping error branch.
func TestNewFromEnv_BadDSNRejected(t *testing.T) {
	t.Setenv("GARUDAAUDIT_DB_URL", "postgres://user:pass@127.0.0.1:1/db?sslmode=disable&connect_timeout=1")
	if _, _, err := NewFromEnv(); err == nil {
		t.Error("expected ping error")
	}
}

// TestNewFromEnv_DataBaseURLFallback pins the secondary env var.
func TestNewFromEnv_DataBaseURLFallback(t *testing.T) {
	t.Setenv("GARUDAAUDIT_DB_URL", "")
	t.Setenv("DATABASE_URL", "postgres://user:pass@127.0.0.1:1/db?sslmode=disable&connect_timeout=1")
	if _, _, err := NewFromEnv(); err == nil {
		t.Error("expected ping error via DATABASE_URL")
	}
}

// TestNew_DispatchesByDB pins the New() helper which selects in-memory
// vs postgres based on whether db is nil.
func TestNew_DispatchesByDB(t *testing.T) {
	if s := New(nil); s == nil {
		t.Error("nil db should yield in-memory store, not nil")
	}
	db, _ := sql.Open("postgres", "postgres://localhost:1/x")
	defer db.Close()
	if s := New(db); s == nil {
		t.Error("non-nil db should yield postgres store, not nil")
	}
}

// TestEnvInt_Branches pins envInt.
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

// TestEnvDuration_Branches pins envDuration.
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

// TestConfigurePool_AppliesEnvOverrides pins configurePool's SetMax* calls.
func TestConfigurePool_AppliesEnvOverrides(t *testing.T) {
	db, err := sql.Open("postgres", "postgres://localhost:1/x")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	t.Setenv("DB_MAX_OPEN_CONNS", "75")
	configurePool(db)
	if db.Stats().MaxOpenConnections != 75 {
		t.Errorf("MaxOpenConnections = %d", db.Stats().MaxOpenConnections)
	}
}
