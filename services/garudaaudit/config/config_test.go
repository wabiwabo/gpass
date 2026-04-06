package config

import (
	"testing"
)

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GARUDAAUDIT_DB_URL", "postgres://localhost/garudaaudit")
	t.Setenv("KAFKA_BROKERS", "localhost:19092")
}

func TestLoad_Success(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != "4010" {
		t.Errorf("expected port 4010, got %s", cfg.Port)
	}
	if cfg.DatabaseURL != "postgres://localhost/garudaaudit" {
		t.Errorf("unexpected database URL: %s", cfg.DatabaseURL)
	}
	if cfg.KafkaBrokers != "localhost:19092" {
		t.Errorf("unexpected kafka brokers: %s", cfg.KafkaBrokers)
	}
	if cfg.RetentionDays != 1825 {
		t.Errorf("expected retention days 1825, got %d", cfg.RetentionDays)
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	// No env vars set — DatabaseURL will be empty
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing required env vars")
	}
}

func TestLoad_CustomRetention(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("GARUDAAUDIT_RETENTION_DAYS", "3650")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.RetentionDays != 3650 {
		t.Errorf("expected retention days 3650, got %d", cfg.RetentionDays)
	}
}

func TestLoad_InvalidRetention(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("GARUDAAUDIT_RETENTION_DAYS", "not-a-number")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid retention days")
	}
}

func TestLoad_ZeroRetention(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("GARUDAAUDIT_RETENTION_DAYS", "0")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for zero retention days")
	}
}

func TestLoad_CustomPort(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("GARUDAAUDIT_PORT", "5050")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != "5050" {
		t.Errorf("expected port 5050, got %s", cfg.Port)
	}
}
