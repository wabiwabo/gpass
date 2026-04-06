package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// Config holds all GarudaInfo service configuration loaded from environment variables.
type Config struct {
	Port               string
	DatabaseURL        string
	IdentityServiceURL string
	KafkaBrokers       string
	OTPRedisURL        string
}

// Load reads configuration from environment variables and validates it.
func Load() (*Config, error) {
	cfg := &Config{
		Port:               getEnv("GARUDAINFO_PORT", "4003"),
		DatabaseURL:        os.Getenv("GARUDAINFO_DB_URL"),
		IdentityServiceURL: os.Getenv("IDENTITY_SERVICE_URL"),
		KafkaBrokers:       getEnv("KAFKA_BROKERS", "localhost:19092"),
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
		{"GARUDAINFO_DB_URL", c.DatabaseURL},
		{"IDENTITY_SERVICE_URL", c.IdentityServiceURL},
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

	// Validate IDENTITY_SERVICE_URL is a valid URL
	if _, err := url.ParseRequestURI(c.IdentityServiceURL); err != nil {
		return fmt.Errorf("invalid URL for IDENTITY_SERVICE_URL: %w", err)
	}

	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
