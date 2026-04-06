// Package envx provides typed environment variable reading
// with defaults, validation, and required variable checks.
// Designed for 12-factor app configuration.
package envx

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// String reads a string environment variable with a default.
func String(key, defaultValue string) string {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	return v
}

// Required reads a required string environment variable.
// Returns an error if not set.
func Required(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("envx: required variable %q not set", key)
	}
	return v, nil
}

// Int reads an integer environment variable with a default.
func Int(key string, defaultValue int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultValue
	}
	return n
}

// Int64 reads an int64 environment variable with a default.
func Int64(key string, defaultValue int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return defaultValue
	}
	return n
}

// Bool reads a boolean environment variable with a default.
// Truthy: "true", "1", "yes", "on" (case-insensitive).
func Bool(key string, defaultValue bool) bool {
	v := strings.ToLower(os.Getenv(key))
	if v == "" {
		return defaultValue
	}
	return v == "true" || v == "1" || v == "yes" || v == "on"
}

// Duration reads a duration environment variable with a default.
func Duration(key string, defaultValue time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return defaultValue
	}
	return d
}

// Float64 reads a float64 environment variable with a default.
func Float64(key string, defaultValue float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return defaultValue
	}
	return f
}

// List reads a comma-separated list environment variable.
func List(key, separator string) []string {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	if separator == "" {
		separator = ","
	}
	parts := strings.Split(v, separator)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// OneOf reads an environment variable that must be one of the allowed values.
func OneOf(key string, allowed []string, defaultValue string) string {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	for _, a := range allowed {
		if v == a {
			return v
		}
	}
	return defaultValue
}

// MustAll checks that all required environment variables are set.
// Returns an error listing all missing variables.
func MustAll(keys ...string) error {
	var missing []string
	for _, k := range keys {
		if os.Getenv(k) == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("envx: missing required variables: %s", strings.Join(missing, ", "))
	}
	return nil
}
