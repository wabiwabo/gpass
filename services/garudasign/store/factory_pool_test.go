package store

import (
	"database/sql"
	"testing"
	"time"
)

func TestNewStoresFromEnv_NoDSNFallsBackToInMemory(t *testing.T) {
	t.Setenv("GARUDASIGN_DB_URL", "")
	t.Setenv("DATABASE_URL", "")
	s, err := NewStoresFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if s.Certificate == nil || s.Request == nil || s.Document == nil {
		t.Fatal("nil store in bundle")
	}
	if s.DB != nil {
		t.Error("DB should be nil for in-memory")
	}
}

func TestNewStoresFromEnv_BadDSNRejected(t *testing.T) {
	t.Setenv("GARUDASIGN_DB_URL", "postgres://user:pass@127.0.0.1:1/db?sslmode=disable&connect_timeout=1")
	if _, err := NewStoresFromEnv(); err == nil {
		t.Error("expected ping error")
	}
}

func TestNewStoresFromEnv_DataBaseURLFallback(t *testing.T) {
	t.Setenv("GARUDASIGN_DB_URL", "")
	t.Setenv("DATABASE_URL", "postgres://user:pass@127.0.0.1:1/db?sslmode=disable&connect_timeout=1")
	if _, err := NewStoresFromEnv(); err == nil {
		t.Error("expected ping error")
	}
}

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
}

func TestEnvDuration_Branches(t *testing.T) {
	t.Setenv("DB_Y", "")
	if envDuration("DB_Y", 30*time.Minute) != 30*time.Minute {
		t.Error("unset")
	}
	t.Setenv("DB_Y", "60")
	if envDuration("DB_Y", 30*time.Minute) != 60*time.Second {
		t.Error("override")
	}
}

func TestConfigurePool_AppliesEnvOverrides(t *testing.T) {
	db, _ := sql.Open("postgres", "postgres://localhost:1/x")
	defer db.Close()
	t.Setenv("DB_MAX_OPEN_CONNS", "75")
	configurePool(db)
	if db.Stats().MaxOpenConnections != 75 {
		t.Errorf("MaxOpenConnections = %d", db.Stats().MaxOpenConnections)
	}
}
