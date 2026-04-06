package config

import (
	"testing"
)

// validEnv sets all required env vars for a valid config.
func validEnv(t *testing.T) {
	t.Helper()
	envs := map[string]string{
		"GARUDACORP_PORT":      "4006",
		"DATABASE_URL":         "postgres://localhost:5432/garudacorp",
		"AHU_URL":              "http://localhost:9091",
		"AHU_API_KEY":          "test-ahu-key",
		"AHU_TIMEOUT":          "10s",
		"OSS_URL":              "http://localhost:9092",
		"OSS_API_KEY":          "test-oss-key",
		"OSS_TIMEOUT":          "10s",
		"IDENTITY_SERVICE_URL": "http://localhost:4001",
		"SERVER_NIK_KEY":       "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"KAFKA_BROKERS":        "localhost:19092",
	}
	for k, v := range envs {
		t.Setenv(k, v)
	}
}

func TestLoad_Success(t *testing.T) {
	validEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "4006" {
		t.Errorf("Port = %q, want %q", cfg.Port, "4006")
	}
	if cfg.AHUURL != "http://localhost:9091" {
		t.Errorf("AHUURL = %q, want %q", cfg.AHUURL, "http://localhost:9091")
	}
	if cfg.OSSURL != "http://localhost:9092" {
		t.Errorf("OSSURL = %q, want %q", cfg.OSSURL, "http://localhost:9092")
	}
	if len(cfg.ServerNIKKey) != 32 {
		t.Errorf("ServerNIKKey length = %d, want 32", len(cfg.ServerNIKKey))
	}
	if cfg.KafkaBrokers != "localhost:19092" {
		t.Errorf("KafkaBrokers = %q, want %q", cfg.KafkaBrokers, "localhost:19092")
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	t.Setenv("SERVER_NIK_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("AHU_URL", "")
	t.Setenv("OSS_URL", "")
	t.Setenv("IDENTITY_SERVICE_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing required vars")
	}
}

func TestLoad_ShortNIKKey(t *testing.T) {
	validEnv(t)
	t.Setenv("SERVER_NIK_KEY", "0123456789abcdef") // too short

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for short NIK key")
	}
}

func TestLoad_InvalidAHUTimeout(t *testing.T) {
	validEnv(t)
	t.Setenv("AHU_TIMEOUT", "not-a-duration")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid AHU timeout")
	}
}

func TestLoad_InvalidOSSTimeout(t *testing.T) {
	validEnv(t)
	t.Setenv("OSS_TIMEOUT", "not-a-duration")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid OSS timeout")
	}
}

func TestLoad_InvalidURL(t *testing.T) {
	validEnv(t)
	t.Setenv("AHU_URL", "://bad-url")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestLoad_DefaultPort(t *testing.T) {
	validEnv(t)
	t.Setenv("GARUDACORP_PORT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "4006" {
		t.Errorf("Port = %q, want default %q", cfg.Port, "4006")
	}
}
