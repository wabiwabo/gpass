package config

import "os"

// Config holds all GarudaNotify service configuration loaded from environment variables.
type Config struct {
	Port         string
	SMTPHost     string
	SMTPPort     string
	SMTPUser     string
	SMTPPassword string
	SMSProvider  string // "mock" or "twilio"
	SMSAPIKey    string
	FromEmail    string
	FromName     string
}

// Load reads configuration from environment variables.
// SMTP and SMS are optional — the service starts in mock mode if not configured.
func Load() (*Config, error) {
	cfg := &Config{
		Port:         getEnv("GARUDANOTIFY_PORT", "4011"),
		SMTPHost:     os.Getenv("SMTP_HOST"),
		SMTPPort:     getEnv("SMTP_PORT", "587"),
		SMTPUser:     os.Getenv("SMTP_USER"),
		SMTPPassword: os.Getenv("SMTP_PASSWORD"),
		SMSProvider:  getEnv("SMS_PROVIDER", "mock"),
		SMSAPIKey:    os.Getenv("SMS_API_KEY"),
		FromEmail:    getEnv("FROM_EMAIL", "noreply@garudapass.id"),
		FromName:     getEnv("FROM_NAME", "GarudaPass"),
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
