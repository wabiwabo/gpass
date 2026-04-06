package environ

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Get returns an environment variable value or a default.
func Get(key, defaultValue string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return defaultValue
}

// GetInt returns an environment variable as int or a default.
func GetInt(key string, defaultValue int) int {
	v, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultValue
	}
	return n
}

// GetBool returns an environment variable as bool or a default.
func GetBool(key string, defaultValue bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return defaultValue
	}
	return b
}

// GetDuration returns an environment variable as time.Duration or a default.
func GetDuration(key string, defaultValue time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return defaultValue
	}
	return d
}

// GetSlice returns an environment variable as a comma-separated string slice.
func GetSlice(key string, defaultValue []string) []string {
	v, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	parts := strings.Split(v, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// Require returns the value of an environment variable or panics if not set.
func Require(key string) string {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		panic("required environment variable not set: " + key)
	}
	return v
}

// IsDevelopment returns true if ENVIRONMENT or GO_ENV is "development".
func IsDevelopment() bool {
	env := Get("ENVIRONMENT", Get("GO_ENV", "development"))
	return env == "development" || env == "dev"
}

// IsProduction returns true if ENVIRONMENT or GO_ENV is "production".
func IsProduction() bool {
	env := Get("ENVIRONMENT", Get("GO_ENV", ""))
	return env == "production" || env == "prod"
}

// IsTest returns true if running in test mode.
func IsTest() bool {
	env := Get("ENVIRONMENT", Get("GO_ENV", ""))
	return env == "test" || env == "testing"
}
