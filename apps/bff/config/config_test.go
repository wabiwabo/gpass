package config_test

import (
	"os"
	"strings"
	"testing"

	"github.com/garudapass/gpass/apps/bff/config"
)

func setTestEnv(t *testing.T) {
	t.Helper()
	envs := map[string]string{
		"BFF_PORT":           "4000",
		"BFF_ENV":            "development",
		"KEYCLOAK_URL":       "http://localhost:8080",
		"KEYCLOAK_REALM":     "garudapass",
		"KEYCLOAK_CLIENT_ID": "bff-client",
		"REDIS_URL":          "redis://localhost:6379",
		"BFF_SESSION_SECRET": "test-secret-must-be-at-least-32-characters-long-ok",
		"BFF_FRONTEND_URL":   "http://localhost:3000",
		"BFF_REDIRECT_URI":   "http://localhost:4000/auth/callback",
	}
	for k, v := range envs {
		os.Setenv(k, v)
	}
	t.Cleanup(func() {
		for k := range envs {
			os.Unsetenv(k)
		}
	})
}

func TestLoadFromEnv(t *testing.T) {
	setTestEnv(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != "4000" {
		t.Errorf("expected port 4000, got %s", cfg.Port)
	}
	if cfg.KeycloakURL != "http://localhost:8080" {
		t.Errorf("expected keycloak URL http://localhost:8080, got %s", cfg.KeycloakURL)
	}
	if cfg.IssuerURL() != "http://localhost:8080/realms/garudapass" {
		t.Errorf("expected issuer URL with realm, got %s", cfg.IssuerURL())
	}
}

func TestLoadMissingRequired(t *testing.T) {
	os.Clearenv()

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for missing required env vars")
	}
}

func TestSessionSecretTooShort(t *testing.T) {
	setTestEnv(t)
	os.Setenv("BFF_SESSION_SECRET", "short")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for short session secret")
	}
	if !strings.Contains(err.Error(), "at least") {
		t.Errorf("expected min-length error, got: %v", err)
	}
}

func TestInvalidURLRejected(t *testing.T) {
	setTestEnv(t)
	os.Setenv("KEYCLOAK_URL", "not-a-valid-url")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
	if !strings.Contains(err.Error(), "invalid URL") {
		t.Errorf("expected URL validation error, got: %v", err)
	}
}

func TestEnvironmentParsing(t *testing.T) {
	setTestEnv(t)

	tests := []struct {
		envVal   string
		wantProd bool
	}{
		{"development", false},
		{"staging", false},
		{"production", true},
		{"prod", true},
	}

	for _, tt := range tests {
		os.Setenv("BFF_ENV", tt.envVal)
		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("env=%s: unexpected error: %v", tt.envVal, err)
		}
		if cfg.IsProd() != tt.wantProd {
			t.Errorf("env=%s: IsProd()=%v, want %v", tt.envVal, cfg.IsProd(), tt.wantProd)
		}
	}
}

func TestIsSecure(t *testing.T) {
	setTestEnv(t)

	os.Setenv("BFF_ENV", "development")
	cfg, _ := config.Load()
	if cfg.IsSecure() {
		t.Error("development should not be secure")
	}

	os.Setenv("BFF_ENV", "staging")
	cfg, _ = config.Load()
	if !cfg.IsSecure() {
		t.Error("staging should be secure")
	}

	os.Setenv("BFF_ENV", "production")
	cfg, _ = config.Load()
	if !cfg.IsSecure() {
		t.Error("production should be secure")
	}
}

func TestTrustedOrigins(t *testing.T) {
	setTestEnv(t)
	os.Setenv("BFF_TRUSTED_ORIGINS", "http://localhost:3000, http://localhost:3001")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.TrustedOrigins) != 2 {
		t.Errorf("expected 2 trusted origins, got %d", len(cfg.TrustedOrigins))
	}
}
