package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all GarudaSign service configuration loaded from environment variables.
type Config struct {
	Port                string
	SigningMode         string
	SigningSimURL       string
	EJBCAURL            string
	EJBCAClientCert     string
	EJBCAClientKey      string
	DSSURL              string
	IdentityServiceURL  string
	DocumentStoragePath string
	MaxSizeMB           int
	RequestTTL          time.Duration
	CertValidityDays    int
	DatabaseURL         string
	KafkaBrokers        string
}

// IsSimulator returns true if the signing mode is "simulator".
func (c *Config) IsSimulator() bool {
	return c.SigningMode == "simulator"
}

// Load reads configuration from environment variables and validates it.
func Load() (*Config, error) {
	maxSize, err := strconv.Atoi(getEnv("GARUDASIGN_MAX_SIZE_MB", "10"))
	if err != nil {
		return nil, fmt.Errorf("invalid GARUDASIGN_MAX_SIZE_MB: %w", err)
	}

	ttl, err := time.ParseDuration(getEnv("GARUDASIGN_REQUEST_TTL", "30m"))
	if err != nil {
		return nil, fmt.Errorf("invalid GARUDASIGN_REQUEST_TTL: %w", err)
	}

	certDays, err := strconv.Atoi(getEnv("GARUDASIGN_CERT_VALIDITY_DAYS", "365"))
	if err != nil {
		return nil, fmt.Errorf("invalid GARUDASIGN_CERT_VALIDITY_DAYS: %w", err)
	}

	cfg := &Config{
		Port:                getEnv("GARUDASIGN_PORT", "4007"),
		SigningMode:         getEnv("GARUDASIGN_SIGNING_MODE", "simulator"),
		SigningSimURL:       os.Getenv("SIGNING_SIM_URL"),
		EJBCAURL:            os.Getenv("EJBCA_URL"),
		EJBCAClientCert:     os.Getenv("EJBCA_CLIENT_CERT"),
		EJBCAClientKey:      os.Getenv("EJBCA_CLIENT_KEY"),
		DSSURL:              os.Getenv("DSS_URL"),
		IdentityServiceURL:  os.Getenv("IDENTITY_SERVICE_URL"),
		DocumentStoragePath: os.Getenv("DOCUMENT_STORAGE_PATH"),
		MaxSizeMB:           maxSize,
		RequestTTL:          ttl,
		CertValidityDays:    certDays,
		DatabaseURL:         os.Getenv("GARUDASIGN_DB_URL"),
		KafkaBrokers:        getEnv("KAFKA_BROKERS", "localhost:19092"),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	// SigningMode must be simulator or real
	if c.SigningMode != "simulator" && c.SigningMode != "real" {
		return fmt.Errorf("GARUDASIGN_SIGNING_MODE must be \"simulator\" or \"real\", got %q", c.SigningMode)
	}

	if c.MaxSizeMB <= 0 {
		return fmt.Errorf("GARUDASIGN_MAX_SIZE_MB must be positive, got %d", c.MaxSizeMB)
	}

	// Mode-specific requirements
	if c.SigningMode == "simulator" {
		if c.SigningSimURL == "" {
			return fmt.Errorf("SIGNING_SIM_URL is required when signing mode is \"simulator\"")
		}
		if err := validateURL("SIGNING_SIM_URL", c.SigningSimURL); err != nil {
			return err
		}
	} else {
		if c.EJBCAURL == "" || c.DSSURL == "" {
			return fmt.Errorf("EJBCA_URL and DSS_URL are required when signing mode is \"real\"")
		}
		if err := validateURL("EJBCA_URL", c.EJBCAURL); err != nil {
			return err
		}
		if err := validateURL("DSS_URL", c.DSSURL); err != nil {
			return err
		}
	}

	// Always required
	required := []struct {
		name  string
		value string
	}{
		{"IDENTITY_SERVICE_URL", c.IdentityServiceURL},
		{"DOCUMENT_STORAGE_PATH", c.DocumentStoragePath},
		{"GARUDASIGN_DB_URL", c.DatabaseURL},
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

	if err := validateURL("IDENTITY_SERVICE_URL", c.IdentityServiceURL); err != nil {
		return err
	}

	return nil
}

func validateURL(name, value string) error {
	if _, err := url.ParseRequestURI(value); err != nil {
		return fmt.Errorf("invalid URL for %s: %w", name, err)
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
