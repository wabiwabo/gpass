package config

import (
	"os"
	"testing"
)

// validEnv sets all required env vars for a valid config.
func validEnv(t *testing.T) {
	t.Helper()
	envs := map[string]string{
		"IDENTITY_PORT":          "4001",
		"DATABASE_URL":           "postgres://localhost:5432/identity",
		"DUKCAPIL_MODE":          "simulator",
		"DUKCAPIL_URL":           "http://localhost:9090",
		"DUKCAPIL_API_KEY":       "",
		"DUKCAPIL_TIMEOUT":       "10s",
		"SERVER_NIK_KEY":         "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"FIELD_ENCRYPTION_KEY":   "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210",
		"KEYCLOAK_ADMIN_URL":     "http://localhost:8080",
		"KEYCLOAK_ADMIN_USER":    "admin",
		"KEYCLOAK_ADMIN_PASSWORD": "admin",
		"KEYCLOAK_REALM":         "garudapass",
		"OTP_REDIS_URL":          "redis://localhost:6379",
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
	if cfg.Port != "4001" {
		t.Errorf("Port = %q, want %q", cfg.Port, "4001")
	}
	if cfg.DukcapilMode != "simulator" {
		t.Errorf("DukcapilMode = %q, want %q", cfg.DukcapilMode, "simulator")
	}
	if cfg.KeycloakRealm != "garudapass" {
		t.Errorf("KeycloakRealm = %q, want %q", cfg.KeycloakRealm, "garudapass")
	}
	if len(cfg.ServerNIKKey) != 32 {
		t.Errorf("ServerNIKKey length = %d, want 32", len(cfg.ServerNIKKey))
	}
	if len(cfg.FieldEncryptionKey) != 32 {
		t.Errorf("FieldEncryptionKey length = %d, want 32", len(cfg.FieldEncryptionKey))
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	// Set keys but not DATABASE_URL
	t.Setenv("SERVER_NIK_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	t.Setenv("FIELD_ENCRYPTION_KEY", "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DUKCAPIL_URL", "")
	t.Setenv("KEYCLOAK_ADMIN_URL", "")
	t.Setenv("KEYCLOAK_ADMIN_USER", "")
	t.Setenv("KEYCLOAK_ADMIN_PASSWORD", "")
	t.Setenv("OTP_REDIS_URL", "")

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

func TestLoad_InvalidMode(t *testing.T) {
	validEnv(t)
	t.Setenv("DUKCAPIL_MODE", "invalid")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

func TestLoad_RealModeRequiresAPIKey(t *testing.T) {
	validEnv(t)
	t.Setenv("DUKCAPIL_MODE", "real")
	os.Unsetenv("DUKCAPIL_API_KEY")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when mode=real and no API key")
	}
}
