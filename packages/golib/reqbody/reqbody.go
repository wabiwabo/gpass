// Package reqbody provides safe HTTP request body reading with
// size limits, content-type validation, and JSON decoding.
// Prevents memory exhaustion from oversized payloads.
package reqbody

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Config controls request body reading.
type Config struct {
	// MaxSize is the maximum body size in bytes.
	MaxSize int64
	// AllowedTypes is the list of allowed Content-Type values.
	// Empty means all types are allowed.
	AllowedTypes []string
}

// DefaultConfig returns production defaults.
func DefaultConfig() Config {
	return Config{
		MaxSize: 1 << 20, // 1 MB
	}
}

// ReadJSON reads and decodes a JSON request body into v.
func ReadJSON(r *http.Request, v interface{}, cfg Config) error {
	if cfg.MaxSize <= 0 {
		cfg.MaxSize = 1 << 20
	}

	// Check content type
	ct := r.Header.Get("Content-Type")
	if ct != "" && !strings.HasPrefix(ct, "application/json") {
		return fmt.Errorf("reqbody: unsupported content type %q, expected application/json", ct)
	}

	// Limit body size
	body := http.MaxBytesReader(nil, r.Body, cfg.MaxSize)
	defer body.Close()

	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(v); err != nil {
		if err.Error() == "http: request body too large" {
			return fmt.Errorf("reqbody: body exceeds maximum size of %d bytes", cfg.MaxSize)
		}
		return fmt.Errorf("reqbody: invalid JSON: %w", err)
	}

	// Ensure no trailing content
	if decoder.More() {
		return fmt.Errorf("reqbody: unexpected data after JSON body")
	}

	return nil
}

// ReadBytes reads the raw request body with size limit.
func ReadBytes(r *http.Request, cfg Config) ([]byte, error) {
	if cfg.MaxSize <= 0 {
		cfg.MaxSize = 1 << 20
	}

	// Check allowed content types
	if len(cfg.AllowedTypes) > 0 {
		ct := r.Header.Get("Content-Type")
		allowed := false
		for _, t := range cfg.AllowedTypes {
			if strings.HasPrefix(ct, t) {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, fmt.Errorf("reqbody: content type %q not allowed", ct)
		}
	}

	body := http.MaxBytesReader(nil, r.Body, cfg.MaxSize)
	defer body.Close()

	data, err := io.ReadAll(body)
	if err != nil {
		if err.Error() == "http: request body too large" {
			return nil, fmt.Errorf("reqbody: body exceeds maximum size of %d bytes", cfg.MaxSize)
		}
		return nil, fmt.Errorf("reqbody: read error: %w", err)
	}

	return data, nil
}

// ContentLength returns the request's Content-Length, or -1 if unknown.
func ContentLength(r *http.Request) int64 {
	return r.ContentLength
}

// HasBody checks if the request likely has a body.
func HasBody(r *http.Request) bool {
	return r.ContentLength > 0 || r.Body != nil
}
