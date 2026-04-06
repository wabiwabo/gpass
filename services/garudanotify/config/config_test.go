package config

import (
	"os"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Port != "4011" {
		t.Errorf("expected default port %q, got %q", "4011", cfg.Port)
	}
	if cfg.SMSProvider != "mock" {
		t.Errorf("expected default SMS provider %q, got %q", "mock", cfg.SMSProvider)
	}
	if cfg.FromEmail != "noreply@garudapass.id" {
		t.Errorf("expected default from email %q, got %q", "noreply@garudapass.id", cfg.FromEmail)
	}
	if cfg.FromName != "GarudaPass" {
		t.Errorf("expected default from name %q, got %q", "GarudaPass", cfg.FromName)
	}
}

func TestLoad_CustomPort(t *testing.T) {
	os.Setenv("GARUDANOTIFY_PORT", "9090")
	defer os.Unsetenv("GARUDANOTIFY_PORT")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Port != "9090" {
		t.Errorf("expected port %q, got %q", "9090", cfg.Port)
	}
}
