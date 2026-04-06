package config

import (
	"os"
	"testing"
)

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GARUDASIGN_SIGNING_MODE", "simulator")
	t.Setenv("SIGNING_SIM_URL", "http://localhost:4008")
	t.Setenv("IDENTITY_SERVICE_URL", "http://localhost:4001")
	t.Setenv("DOCUMENT_STORAGE_PATH", "/data/signing")
	t.Setenv("GARUDASIGN_DB_URL", "postgres://localhost/garudasign")
}

func TestLoad_Success(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != "4007" {
		t.Errorf("expected port 4007, got %s", cfg.Port)
	}
	if cfg.SigningMode != "simulator" {
		t.Errorf("expected simulator mode, got %s", cfg.SigningMode)
	}
	if !cfg.IsSimulator() {
		t.Error("expected IsSimulator() to return true")
	}
	if cfg.MaxSizeMB != 10 {
		t.Errorf("expected max size 10, got %d", cfg.MaxSizeMB)
	}
	if cfg.CertValidityDays != 365 {
		t.Errorf("expected cert validity 365, got %d", cfg.CertValidityDays)
	}
}

func TestLoad_InvalidMode(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("GARUDASIGN_SIGNING_MODE", "invalid")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

func TestLoad_RealModeRequiresEJBCA(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("GARUDASIGN_SIGNING_MODE", "real")
	os.Unsetenv("EJBCA_URL")
	os.Unsetenv("DSS_URL")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for real mode without EJBCA_URL")
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	t.Setenv("GARUDASIGN_SIGNING_MODE", "simulator")
	t.Setenv("SIGNING_SIM_URL", "http://localhost:4008")
	// Missing IDENTITY_SERVICE_URL, DOCUMENT_STORAGE_PATH, GARUDASIGN_DB_URL

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing required env vars")
	}
}

func TestLoad_InvalidURL(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("IDENTITY_SERVICE_URL", "not-a-url")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestLoad_InvalidMaxSize(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("GARUDASIGN_MAX_SIZE_MB", "0")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for zero max size")
	}
}
