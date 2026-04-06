package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all GarudaAudit service configuration loaded from environment variables.
type Config struct {
	Port          string
	DatabaseURL   string
	KafkaBrokers  string
	RetentionDays int
}

// Load reads configuration from environment variables and validates it.
func Load() (*Config, error) {
	retentionDays, err := strconv.Atoi(getEnv("GARUDAAUDIT_RETENTION_DAYS", "1825"))
	if err != nil {
		return nil, fmt.Errorf("invalid GARUDAAUDIT_RETENTION_DAYS: %w", err)
	}

	cfg := &Config{
		Port:          getEnv("GARUDAAUDIT_PORT", "4010"),
		DatabaseURL:   os.Getenv("GARUDAAUDIT_DB_URL"),
		KafkaBrokers:  getEnv("KAFKA_BROKERS", "localhost:19092"),
		RetentionDays: retentionDays,
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	var missing []string

	if c.DatabaseURL == "" {
		missing = append(missing, "GARUDAAUDIT_DB_URL")
	}
	if c.KafkaBrokers == "" {
		missing = append(missing, "KAFKA_BROKERS")
	}

	if len(missing) > 0 {
		return fmt.Errorf("required environment variables not set: %s", strings.Join(missing, ", "))
	}

	if c.RetentionDays <= 0 {
		return fmt.Errorf("GARUDAAUDIT_RETENTION_DAYS must be positive, got %d", c.RetentionDays)
	}

	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
