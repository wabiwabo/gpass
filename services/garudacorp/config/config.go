package config

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

// Config holds all GarudaCorp service configuration loaded from environment variables.
type Config struct {
	Port               string
	DatabaseURL        string
	AHUURL             string
	AHUAPIKey          string
	AHUTimeout         time.Duration
	OSSURL             string
	OSSAPIKey          string
	OSSTimeout         time.Duration
	IdentityServiceURL string
	ServerNIKKey       []byte // 32 bytes decoded from hex
	KafkaBrokers       string
}

// Load reads configuration from environment variables and validates it.
func Load() (*Config, error) {
	ahuTimeoutStr := getEnv("AHU_TIMEOUT", "10s")
	ahuTimeout, err := time.ParseDuration(ahuTimeoutStr)
	if err != nil {
		return nil, fmt.Errorf("invalid AHU_TIMEOUT %q: %w", ahuTimeoutStr, err)
	}

	ossTimeoutStr := getEnv("OSS_TIMEOUT", "10s")
	ossTimeout, err := time.ParseDuration(ossTimeoutStr)
	if err != nil {
		return nil, fmt.Errorf("invalid OSS_TIMEOUT %q: %w", ossTimeoutStr, err)
	}

	nikKeyHex := os.Getenv("SERVER_NIK_KEY")
	nikKey, err := decodeHexKey(nikKeyHex, "SERVER_NIK_KEY", 32)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Port:               getEnv("GARUDACORP_PORT", "4006"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		AHUURL:             os.Getenv("AHU_URL"),
		AHUAPIKey:          os.Getenv("AHU_API_KEY"),
		AHUTimeout:         ahuTimeout,
		OSSURL:             os.Getenv("OSS_URL"),
		OSSAPIKey:          os.Getenv("OSS_API_KEY"),
		OSSTimeout:         ossTimeout,
		IdentityServiceURL: os.Getenv("IDENTITY_SERVICE_URL"),
		ServerNIKKey:       nikKey,
		KafkaBrokers:       getEnv("KAFKA_BROKERS", "localhost:19092"),
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
		{"AHU_URL", c.AHUURL},
		{"OSS_URL", c.OSSURL},
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

	// Validate URLs
	for _, check := range []struct {
		name, val string
	}{
		{"DATABASE_URL", c.DatabaseURL},
		{"AHU_URL", c.AHUURL},
		{"OSS_URL", c.OSSURL},
		{"IDENTITY_SERVICE_URL", c.IdentityServiceURL},
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
