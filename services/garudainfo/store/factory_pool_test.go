package store

import (
	"database/sql"
	"testing"
	"time"
)

func TestNewConsentStoreFromEnv_NoDSNFallsBackToInMemory(t *testing.T) {
	t.Setenv("GARUDAINFO_DB_URL", "")
	t.Setenv("DATABASE_URL", "")
	s, db, err := NewConsentStoreFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if s == nil {
		t.Fatal("nil store")
	}
	if db != nil {
		t.Error("DB should be nil")
	}
}

func TestNewConsentStoreFromEnv_BadDSNRejected(t *testing.T) {
	t.Setenv("GARUDAINFO_DB_URL", "postgres://user:pass@127.0.0.1:1/db?sslmode=disable&connect_timeout=1")
	if _, _, err := NewConsentStoreFromEnv(); err == nil {
		t.Error("expected ping error")
	}
}

func TestNewConsentStoreFromEnv_DataBaseURLFallback(t *testing.T) {
	t.Setenv("GARUDAINFO_DB_URL", "")
	t.Setenv("DATABASE_URL", "postgres://user:pass@127.0.0.1:1/db?sslmode=disable&connect_timeout=1")
	if _, _, err := NewConsentStoreFromEnv(); err == nil {
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
