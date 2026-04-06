package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// Config holds all GarudaPortal service configuration loaded from environment variables.
type Config struct {
	Port               string
	DatabaseURL        string
	IdentityServiceURL string
	KeycloakAdminURL   string
	KeycloakAdminUser  string
	KeycloakAdminPass  string
	KafkaBrokers       string
	RedisURL           string
}

// Load reads configuration from environment variables and validates it.
func Load() (*Config, error) {
	cfg := &Config{
		Port:               getEnv("GARUDAPORTAL_PORT", "4009"),
		DatabaseURL:        os.Getenv("GARUDAPORTAL_DB_URL"),
		IdentityServiceURL: os.Getenv("IDENTITY_SERVICE_URL"),
		KeycloakAdminURL:   os.Getenv("KEYCLOAK_ADMIN_URL"),
		KeycloakAdminUser:  os.Getenv("KEYCLOAK_ADMIN_USER"),
		KeycloakAdminPass:  os.Getenv("KEYCLOAK_ADMIN_PASSWORD"),
		KafkaBrokers:       getEnv("KAFKA_BROKERS", "localhost:19092"),
		RedisURL:           os.Getenv("REDIS_URL"),
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
		{"GARUDAPORTAL_DB_URL", c.DatabaseURL},
		{"IDENTITY_SERVICE_URL", c.IdentityServiceURL},
		{"KEYCLOAK_ADMIN_URL", c.KeycloakAdminURL},
		{"KEYCLOAK_ADMIN_USER", c.KeycloakAdminUser},
		{"KEYCLOAK_ADMIN_PASSWORD", c.KeycloakAdminPass},
		{"REDIS_URL", c.RedisURL},
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

	// Validate URLs
	for _, check := range []struct {
		name, val string
	}{
		{"GARUDAPORTAL_DB_URL", c.DatabaseURL},
		{"IDENTITY_SERVICE_URL", c.IdentityServiceURL},
		{"KEYCLOAK_ADMIN_URL", c.KeycloakAdminURL},
		{"REDIS_URL", c.RedisURL},
	} {
		if _, err := url.ParseRequestURI(check.val); err != nil {
			return fmt.Errorf("invalid URL for %s: %w", check.name, err)
		}
	}

	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
