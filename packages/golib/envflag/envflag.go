// Package envflag provides environment variable to flag bridging.
// Reads configuration from environment variables with fallback defaults,
// type-safe parsing, and prefix support for namespacing.
package envflag

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// String reads an env var as string with fallback.
func String(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Int reads an env var as int with fallback.
func Int(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

// Int64 reads an env var as int64 with fallback.
func Int64(key string, fallback int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fallback
	}
	return n
}

// Bool reads an env var as bool with fallback.
// Truthy: "1", "true", "yes", "on" (case-insensitive).
func Bool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return fallback
}

// Duration reads an env var as time.Duration with fallback.
func Duration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

// Float64 reads an env var as float64 with fallback.
func Float64(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}

// StringSlice reads an env var as comma-separated strings.
func StringSlice(key, sep string, fallback []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parts := strings.Split(v, sep)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return fallback
	}
	return result
}

// MustString reads an env var, panicking if not set.
func MustString(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic("envflag: required env var not set: " + key)
	}
	return v
}

// Prefix returns a reader that prepends a prefix to all keys.
type Prefix struct {
	prefix string
}

// NewPrefix creates a prefixed env reader.
func NewPrefix(prefix string) Prefix {
	if !strings.HasSuffix(prefix, "_") {
		prefix += "_"
	}
	return Prefix{prefix: prefix}
}

// String reads a prefixed env var.
func (p Prefix) String(key, fallback string) string {
	return String(p.prefix+key, fallback)
}

// Int reads a prefixed env var as int.
func (p Prefix) Int(key string, fallback int) int {
	return Int(p.prefix+key, fallback)
}

// Bool reads a prefixed env var as bool.
func (p Prefix) Bool(key string, fallback bool) bool {
	return Bool(p.prefix+key, fallback)
}

// Duration reads a prefixed env var as duration.
func (p Prefix) Duration(key string, fallback time.Duration) time.Duration {
	return Duration(p.prefix+key, fallback)
}
