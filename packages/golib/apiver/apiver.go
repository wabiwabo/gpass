// Package apiver provides HTTP API versioning via URL path prefix,
// header, or query parameter. Supports version deprecation warnings
// and minimum version enforcement.
package apiver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// Version represents an API version.
type Version struct {
	Major int
	Minor int
}

// String returns "vMAJOR.MINOR" or "vMAJOR" if Minor is 0.
func (v Version) String() string {
	if v.Minor > 0 {
		return fmt.Sprintf("v%d.%d", v.Major, v.Minor)
	}
	return fmt.Sprintf("v%d", v.Major)
}

// LessThan checks if v is older than other.
func (v Version) LessThan(other Version) bool {
	if v.Major != other.Major {
		return v.Major < other.Major
	}
	return v.Minor < other.Minor
}

// Equal checks version equality.
func (v Version) Equal(other Version) bool {
	return v.Major == other.Major && v.Minor == other.Minor
}

// Parse parses a version string like "v1", "v2.1", "1", "2.1".
func Parse(s string) (Version, error) {
	s = strings.TrimPrefix(s, "v")
	s = strings.TrimPrefix(s, "V")

	parts := strings.SplitN(s, ".", 2)
	major, err := strconv.Atoi(parts[0])
	if err != nil || major < 0 {
		return Version{}, fmt.Errorf("apiver: invalid version %q", s)
	}

	minor := 0
	if len(parts) == 2 {
		minor, err = strconv.Atoi(parts[1])
		if err != nil || minor < 0 {
			return Version{}, fmt.Errorf("apiver: invalid minor version %q", parts[1])
		}
	}

	return Version{Major: major, Minor: minor}, nil
}

// Config controls versioning behavior.
type Config struct {
	Current    Version   // Current/latest version.
	Minimum    Version   // Minimum supported version.
	Deprecated []Version // Deprecated versions (still work, emit warning).
	// Source determines where to look for the version.
	// "path" (default), "header", or "query".
	Source     string
	HeaderName string // Header name for "header" source (default: "API-Version").
	QueryParam string // Query param for "query" source (default: "version").
}

// DefaultConfig returns a sensible config.
func DefaultConfig() Config {
	return Config{
		Current:    Version{Major: 1},
		Minimum:    Version{Major: 1},
		Source:     "header",
		HeaderName: "API-Version",
		QueryParam: "version",
	}
}

// Extract extracts the version from a request based on config.
func Extract(r *http.Request, cfg Config) (Version, error) {
	var raw string

	switch cfg.Source {
	case "header":
		name := cfg.HeaderName
		if name == "" {
			name = "API-Version"
		}
		raw = r.Header.Get(name)
	case "query":
		param := cfg.QueryParam
		if param == "" {
			param = "version"
		}
		raw = r.URL.Query().Get(param)
	case "path":
		// Extract from first path segment: /v1/users → v1
		path := strings.TrimPrefix(r.URL.Path, "/")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) > 0 {
			raw = parts[0]
		}
	default:
		raw = r.Header.Get("API-Version")
	}

	if raw == "" {
		return cfg.Current, nil // Default to current.
	}

	return Parse(raw)
}

// Middleware validates API version and sets deprecation headers.
func Middleware(cfg Config) func(http.Handler) http.Handler {
	deprecatedSet := make(map[string]bool)
	for _, v := range cfg.Deprecated {
		deprecatedSet[v.String()] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ver, err := Extract(r, cfg)
			if err != nil {
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusBadRequest)
				// Use json.Marshal on the caller-supplied error text so that
				// any quotes/backslashes are escaped — writing it with a
				// raw %s would break the JSON envelope and open a reflected
				// injection path through the Accept-Version header.
				detail, _ := json.Marshal(err.Error())
				fmt.Fprintf(w, `{"type":"about:blank","title":"Invalid API Version","status":400,"detail":%s}`, detail)
				return
			}

			// Check minimum version.
			if ver.LessThan(cfg.Minimum) {
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusGone)
				fmt.Fprintf(w, `{"type":"about:blank","title":"API Version Not Supported","status":410,"detail":"Minimum version is %s"}`, cfg.Minimum)
				return
			}

			// Check deprecation.
			if deprecatedSet[ver.String()] {
				w.Header().Set("Deprecation", "true")
				w.Header().Set("Sunset", "")
				w.Header().Set("Link", fmt.Sprintf(`</api/%s>; rel="successor-version"`, cfg.Current))
			}

			// Set version in response header.
			w.Header().Set("API-Version", ver.String())

			next.ServeHTTP(w, r)
		})
	}
}
