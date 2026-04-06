package config

import (
	"os"
	"strings"
	"testing"
)

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GARUDAPORTAL_DB_URL", "postgres://user:pass@localhost:5432/garudapass")
	t.Setenv("IDENTITY_SERVICE_URL", "http://localhost:4001")
	t.Setenv("KEYCLOAK_ADMIN_URL", "http://localhost:8080")
	t.Setenv("KEYCLOAK_ADMIN_USER", "admin")
	t.Setenv("KEYCLOAK_ADMIN_PASSWORD", "admin")
	t.Setenv("REDIS_URL", "redis://:pass@localhost:6379")
}

func TestLoad_Success(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != "4009" {
		t.Errorf("expected port 4009, got %s", cfg.Port)
	}
	if cfg.DatabaseURL != "postgres://user:pass@localhost:5432/garudapass" {
		t.Errorf("unexpected DatabaseURL: %s", cfg.DatabaseURL)
	}
	if cfg.KafkaBrokers != "localhost:19092" {
		t.Errorf("expected default kafka brokers, got %s", cfg.KafkaBrokers)
	}
}

func TestLoad_CustomPort(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("GARUDAPORTAL_PORT", "5000")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != "5000" {
		t.Errorf("expected port 5000, got %s", cfg.Port)
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	// Clear all env vars
	for _, key := range []string{
		"GARUDAPORTAL_DB_URL", "IDENTITY_SERVICE_URL",
		"KEYCLOAK_ADMIN_URL", "KEYCLOAK_ADMIN_USER",
		"KEYCLOAK_ADMIN_PASSWORD", "REDIS_URL",
	} {
		os.Unsetenv(key)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing env vars")
	}
	if !strings.Contains(err.Error(), "required environment variables not set") {
		t.Errorf("unexpected error message: %v", err)
	}
	if !strings.Contains(err.Error(), "GARUDAPORTAL_DB_URL") {
		t.Errorf("error should mention GARUDAPORTAL_DB_URL: %v", err)
	}
}

func TestLoad_InvalidURL(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("GARUDAPORTAL_DB_URL", "not-a-url")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
	if !strings.Contains(err.Error(), "invalid URL") {
		t.Errorf("unexpected error: %v", err)
	}
}
