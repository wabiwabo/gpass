package config

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

// Config holds all Identity service configuration loaded from environment variables.
type Config struct {
	Port               string
	DatabaseURL        string
	DukcapilMode       string // "simulator" or "real"
	DukcapilURL        string
	DukcapilAPIKey     string
	DukcapilTimeout    time.Duration
	ServerNIKKey       []byte // 32 bytes decoded from hex
	FieldEncryptionKey []byte // 32 bytes decoded from hex
	KeycloakAdminURL   string
	KeycloakAdminUser  string
	KeycloakAdminPass  string
	KeycloakRealm      string
	OTPRedisURL        string
}

// Load reads configuration from environment variables and validates it.
func Load() (*Config, error) {
	timeoutStr := getEnv("DUKCAPIL_TIMEOUT", "10s")
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return nil, fmt.Errorf("invalid DUKCAPIL_TIMEOUT %q: %w", timeoutStr, err)
	}

	nikKeyHex := os.Getenv("SERVER_NIK_KEY")
	nikKey, err := decodeHexKey(nikKeyHex, "SERVER_NIK_KEY", 32)
	if err != nil {
		return nil, err
	}

	fieldKeyHex := os.Getenv("FIELD_ENCRYPTION_KEY")
	fieldKey, err := decodeHexKey(fieldKeyHex, "FIELD_ENCRYPTION_KEY", 32)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Port:               getEnv("IDENTITY_PORT", "4001"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		DukcapilMode:       getEnv("DUKCAPIL_MODE", "simulator"),
		DukcapilURL:        os.Getenv("DUKCAPIL_URL"),
		DukcapilAPIKey:     os.Getenv("DUKCAPIL_API_KEY"),
		DukcapilTimeout:    timeout,
		ServerNIKKey:       nikKey,
		FieldEncryptionKey: fieldKey,
		KeycloakAdminURL:   os.Getenv("KEYCLOAK_ADMIN_URL"),
		KeycloakAdminUser:  os.Getenv("KEYCLOAK_ADMIN_USER"),
		KeycloakAdminPass:  os.Getenv("KEYCLOAK_ADMIN_PASSWORD"),
		KeycloakRealm:      getEnv("KEYCLOAK_REALM", "garudapass"),
		OTPRedisURL:        os.Getenv("OTP_REDIS_URL"),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	required := []struct {
		name  string
		value string
	}{
		{"DATABASE_URL", c.DatabaseURL},
		{"DUKCAPIL_URL", c.DukcapilURL},
		{"KEYCLOAK_ADMIN_URL", c.KeycloakAdminURL},
		{"KEYCLOAK_ADMIN_USER", c.KeycloakAdminUser},
		{"KEYCLOAK_ADMIN_PASSWORD", c.KeycloakAdminPass},
		{"OTP_REDIS_URL", c.OTPRedisURL},
	}

	var missing []string
	for _, r := range required {
		if r.value == "" {
			missing = append(missing, r.name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("required environment variables not set: %s", strings.Join(missing, ", "))
	}

	// Validate Dukcapil mode
	switch c.DukcapilMode {
	case "simulator", "real":
	default:
		return fmt.Errorf("invalid DUKCAPIL_MODE %q: must be \"simulator\" or \"real\"", c.DukcapilMode)
	}

	// When mode is real, API key is required
	if c.DukcapilMode == "real" && c.DukcapilAPIKey == "" {
		return fmt.Errorf("DUKCAPIL_API_KEY is required when DUKCAPIL_MODE=real")
	}

	// Validate URLs
	for _, check := range []struct {
		name, val string
	}{
		{"DATABASE_URL", c.DatabaseURL},
		{"DUKCAPIL_URL", c.DukcapilURL},
		{"KEYCLOAK_ADMIN_URL", c.KeycloakAdminURL},
		{"OTP_REDIS_URL", c.OTPRedisURL},
	} {
		if _, err := url.ParseRequestURI(check.val); err != nil {
			return fmt.Errorf("invalid URL for %s: %w", check.name, err)
		}
	}

	return nil
}

func decodeHexKey(hexStr, name string, expectedLen int) ([]byte, error) {
	if hexStr == "" {
		return nil, fmt.Errorf("required environment variable not set: %s", name)
	}
	if len(hexStr) != expectedLen*2 {
		return nil, fmt.Errorf("%s must be %d hex characters (got %d)", name, expectedLen*2, len(hexStr))
	}
	key, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hex for %s: %w", name, err)
	}
	return key, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
