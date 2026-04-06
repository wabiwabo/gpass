package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// Environment represents the deployment environment.
type Environment string

const (
	EnvDevelopment Environment = "development"
	EnvStaging     Environment = "staging"
	EnvProduction  Environment = "production"

	minSessionSecretLen = 32
)

// Config holds all BFF configuration loaded from environment variables.
type Config struct {
	Port          string
	Environment   Environment
	KeycloakURL   string
	KeycloakRealm string
	ClientID      string
	RedisURL      string
	SessionSecret string
	FrontendURL   string
	RedirectURI   string
	CookieDomain  string
	TrustedOrigins []string
}

// IssuerURL returns the OIDC issuer URL for the configured realm.
func (c *Config) IssuerURL() string {
	return fmt.Sprintf("%s/realms/%s", c.KeycloakURL, c.KeycloakRealm)
}

// IsProd returns true if the environment is production.
func (c *Config) IsProd() bool {
	return c.Environment == EnvProduction
}

// IsSecure returns true if cookies should be sent with the Secure flag.
// Enabled for production and staging.
func (c *Config) IsSecure() bool {
	return c.Environment != EnvDevelopment
}

// Load reads configuration from environment variables and validates it.
func Load() (*Config, error) {
	env := parseEnvironment(getEnv("BFF_ENV", "development"))

	cfg := &Config{
		Port:          getEnv("BFF_PORT", "4000"),
		Environment:   env,
		KeycloakURL:   os.Getenv("KEYCLOAK_URL"),
		KeycloakRealm: os.Getenv("KEYCLOAK_REALM"),
		ClientID:      os.Getenv("KEYCLOAK_CLIENT_ID"),
		RedisURL:      getEnv("REDIS_URL", "redis://localhost:6379"),
		SessionSecret: os.Getenv("BFF_SESSION_SECRET"),
		FrontendURL:   os.Getenv("BFF_FRONTEND_URL"),
		RedirectURI:   os.Getenv("BFF_REDIRECT_URI"),
		CookieDomain:  getEnv("BFF_COOKIE_DOMAIN", "localhost"),
	}

	origins := os.Getenv("BFF_TRUSTED_ORIGINS")
	if origins != "" {
		cfg.TrustedOrigins = strings.Split(origins, ",")
		for i := range cfg.TrustedOrigins {
			cfg.TrustedOrigins[i] = strings.TrimSpace(cfg.TrustedOrigins[i])
		}
	} else {
		cfg.TrustedOrigins = []string{cfg.FrontendURL}
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
		{"KEYCLOAK_URL", c.KeycloakURL},
		{"KEYCLOAK_REALM", c.KeycloakRealm},
		{"KEYCLOAK_CLIENT_ID", c.ClientID},
		{"BFF_SESSION_SECRET", c.SessionSecret},
		{"BFF_FRONTEND_URL", c.FrontendURL},
		{"BFF_REDIRECT_URI", c.RedirectURI},
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

	if len(c.SessionSecret) < minSessionSecretLen {
		return fmt.Errorf("BFF_SESSION_SECRET must be at least %d characters (got %d)", minSessionSecretLen, len(c.SessionSecret))
	}

	for _, check := range []struct {
		name, val string
	}{
		{"KEYCLOAK_URL", c.KeycloakURL},
		{"BFF_FRONTEND_URL", c.FrontendURL},
		{"BFF_REDIRECT_URI", c.RedirectURI},
		{"REDIS_URL", c.RedisURL},
	} {
		if _, err := url.ParseRequestURI(check.val); err != nil {
			return fmt.Errorf("invalid URL for %s: %w", check.name, err)
		}
	}

	return nil
}

func parseEnvironment(s string) Environment {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "production", "prod":
		return EnvProduction
	case "staging", "stg":
		return EnvStaging
	default:
		return EnvDevelopment
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
