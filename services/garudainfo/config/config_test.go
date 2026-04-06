package config

import (
	"os"
	"testing"
)

func setEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for k, v := range env {
		t.Setenv(k, v)
	}
}

func TestLoad_Success(t *testing.T) {
	setEnv(t, map[string]string{
		"GARUDAINFO_PORT":      "5000",
		"GARUDAINFO_DB_URL":    "postgres://localhost:5432/garudainfo",
		"IDENTITY_SERVICE_URL": "http://localhost:4001",
		"KAFKA_BROKERS":        "broker1:9092,broker2:9092",
		"OTP_REDIS_URL":        "redis://localhost:6379",
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "5000" {
		t.Errorf("Port = %q, want %q", cfg.Port, "5000")
	}
	if cfg.DatabaseURL != "postgres://localhost:5432/garudainfo" {
		t.Errorf("DatabaseURL = %q, want postgres URL", cfg.DatabaseURL)
	}
	if cfg.IdentityServiceURL != "http://localhost:4001" {
		t.Errorf("IdentityServiceURL = %q, want http://localhost:4001", cfg.IdentityServiceURL)
	}
	if cfg.KafkaBrokers != "broker1:9092,broker2:9092" {
		t.Errorf("KafkaBrokers = %q, want broker1:9092,broker2:9092", cfg.KafkaBrokers)
	}
}

func TestLoad_Defaults(t *testing.T) {
	setEnv(t, map[string]string{
		"GARUDAINFO_DB_URL":    "postgres://localhost:5432/garudainfo",
		"IDENTITY_SERVICE_URL": "http://localhost:4001",
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "4003" {
		t.Errorf("Port = %q, want default %q", cfg.Port, "4003")
	}
	if cfg.KafkaBrokers != "localhost:19092" {
		t.Errorf("KafkaBrokers = %q, want default %q", cfg.KafkaBrokers, "localhost:19092")
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	// Clear all relevant env vars
	os.Unsetenv("GARUDAINFO_DB_URL")
	os.Unsetenv("IDENTITY_SERVICE_URL")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing required vars, got nil")
	}

	want := "required environment variables not set"
	if got := err.Error(); !contains(got, want) {
		t.Errorf("error = %q, want it to contain %q", got, want)
	}
}

func TestLoad_MissingDBURL(t *testing.T) {
	setEnv(t, map[string]string{
		"IDENTITY_SERVICE_URL": "http://localhost:4001",
	})

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing GARUDAINFO_DB_URL")
	}
}

func TestLoad_InvalidIdentityURL(t *testing.T) {
	setEnv(t, map[string]string{
		"GARUDAINFO_DB_URL":    "postgres://localhost:5432/garudainfo",
		"IDENTITY_SERVICE_URL": "://bad-url",
	})

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid IDENTITY_SERVICE_URL")
	}

	want := "invalid URL for IDENTITY_SERVICE_URL"
	if got := err.Error(); !contains(got, want) {
		t.Errorf("error = %q, want it to contain %q", got, want)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
