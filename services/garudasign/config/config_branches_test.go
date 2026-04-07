package config

import (
	"strings"
	"testing"
)

// setMinimalEnv populates a valid simulator-mode config so individual
// tests can selectively unset/override one variable.
func setMinimalEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GARUDASIGN_PORT", "4007")
	t.Setenv("GARUDASIGN_SIGNING_MODE", "simulator")
	t.Setenv("SIGNING_SIM_URL", "http://signing-sim:4008")
	t.Setenv("EJBCA_URL", "")
	t.Setenv("DSS_URL", "")
	t.Setenv("IDENTITY_SERVICE_URL", "http://identity:4001")
	t.Setenv("DOCUMENT_STORAGE_PATH", "/var/garudasign")
	t.Setenv("GARUDASIGN_DB_URL", "postgres://localhost/x")
	t.Setenv("GARUDASIGN_MAX_SIZE_MB", "10")
	t.Setenv("GARUDASIGN_REQUEST_TTL", "30m")
	t.Setenv("GARUDASIGN_CERT_VALIDITY_DAYS", "365")
}

// TestLoad_HappyPath_SimulatorMode pins the canonical simulator config.
func TestLoad_HappyPath_SimulatorMode(t *testing.T) {
	setMinimalEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.IsSimulator() {
		t.Error("IsSimulator should be true")
	}
	if cfg.MaxSizeMB != 10 {
		t.Errorf("MaxSizeMB = %d", cfg.MaxSizeMB)
	}
}

// TestLoad_RealMode_RequiresEJBCAAndDSS pins the real-mode validation.
func TestLoad_RealMode_RequiresEJBCAAndDSS(t *testing.T) {
	setMinimalEnv(t)
	t.Setenv("GARUDASIGN_SIGNING_MODE", "real")

	// Missing both → error.
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "EJBCA_URL and DSS_URL") {
		t.Errorf("err = %v", err)
	}

	// Provide both → success.
	t.Setenv("EJBCA_URL", "https://ejbca.example/api")
	t.Setenv("DSS_URL", "https://dss.example/api")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.IsSimulator() {
		t.Error("real mode should not report IsSimulator")
	}
}

// TestLoad_BadParseErrors pins the strconv/time.ParseDuration error
// branches in Load.
func TestLoad_BadParseErrors(t *testing.T) {
	setMinimalEnv(t)
	t.Setenv("GARUDASIGN_MAX_SIZE_MB", "not-a-number")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "MAX_SIZE_MB") {
		t.Errorf("max size: %v", err)
	}

	setMinimalEnv(t)
	t.Setenv("GARUDASIGN_REQUEST_TTL", "10potatoes")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "REQUEST_TTL") {
		t.Errorf("ttl: %v", err)
	}

	setMinimalEnv(t)
	t.Setenv("GARUDASIGN_CERT_VALIDITY_DAYS", "abc")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "CERT_VALIDITY_DAYS") {
		t.Errorf("cert days: %v", err)
	}
}

// TestValidate_BadSigningMode pins the mode-allowlist branch.
func TestValidate_BadSigningMode(t *testing.T) {
	setMinimalEnv(t)
	t.Setenv("GARUDASIGN_SIGNING_MODE", "production")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "must be") {
		t.Errorf("err = %v", err)
	}
}

// TestValidate_NegativeMaxSize pins the MaxSizeMB<=0 branch.
func TestValidate_NegativeMaxSize(t *testing.T) {
	setMinimalEnv(t)
	t.Setenv("GARUDASIGN_MAX_SIZE_MB", "0")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "positive") {
		t.Errorf("err = %v", err)
	}
}

// TestValidate_MissingRequired pins the multi-field missing-vars branch.
func TestValidate_MissingRequired(t *testing.T) {
	setMinimalEnv(t)
	t.Setenv("IDENTITY_SERVICE_URL", "")
	t.Setenv("DOCUMENT_STORAGE_PATH", "")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "IDENTITY_SERVICE_URL") {
		t.Errorf("err = %v", err)
	}
}

// TestValidate_MalformedURL pins the validateURL branch — a value that
// is set but isn't a valid URL must be rejected.
func TestValidate_MalformedURL(t *testing.T) {
	setMinimalEnv(t)
	t.Setenv("SIGNING_SIM_URL", "not://a valid url")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "invalid URL") {
		t.Errorf("err = %v", err)
	}
}

// TestValidate_SimulatorRequiresSimURL pins the missing-SIGNING_SIM_URL
// branch in simulator mode.
func TestValidate_SimulatorRequiresSimURL(t *testing.T) {
	setMinimalEnv(t)
	t.Setenv("SIGNING_SIM_URL", "")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "SIGNING_SIM_URL") {
		t.Errorf("err = %v", err)
	}
}
