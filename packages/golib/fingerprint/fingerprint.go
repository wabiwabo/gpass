package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sort"
	"strings"
)

// Fingerprint represents a unique request fingerprint for bot/abuse detection.
type Fingerprint struct {
	Hash       string            `json:"hash"`
	Components map[string]string `json:"components"`
}

// Config configures which request attributes to include in fingerprinting.
type Config struct {
	// IncludeHeaders lists headers to include (case-insensitive).
	IncludeHeaders []string
	// IncludeIP includes the client IP.
	IncludeIP bool
	// IncludeMethod includes the HTTP method.
	IncludeMethod bool
	// IncludePath includes the URL path.
	IncludePath bool
	// IncludeAcceptHeaders includes Accept, Accept-Language, Accept-Encoding.
	IncludeAcceptHeaders bool
}

// DefaultConfig returns a fingerprint config suitable for bot detection.
func DefaultConfig() Config {
	return Config{
		IncludeHeaders: []string{
			"User-Agent",
			"Accept",
			"Accept-Language",
			"Accept-Encoding",
		},
		IncludeIP:            true,
		IncludeMethod:        false,
		IncludePath:          false,
		IncludeAcceptHeaders: true,
	}
}

// Generate creates a fingerprint from the HTTP request.
func Generate(r *http.Request, cfg Config) Fingerprint {
	components := make(map[string]string)

	if cfg.IncludeIP {
		components["ip"] = extractIP(r)
	}
	if cfg.IncludeMethod {
		components["method"] = r.Method
	}
	if cfg.IncludePath {
		components["path"] = r.URL.Path
	}

	for _, h := range cfg.IncludeHeaders {
		val := r.Header.Get(h)
		if val != "" {
			components["header:"+strings.ToLower(h)] = val
		}
	}

	// Compute deterministic hash.
	hash := computeHash(components)

	return Fingerprint{
		Hash:       hash,
		Components: components,
	}
}

// Match checks if two fingerprints are identical.
func (f Fingerprint) Match(other Fingerprint) bool {
	return f.Hash == other.Hash
}

// Similarity returns a similarity score (0.0-1.0) between two fingerprints.
func (f Fingerprint) Similarity(other Fingerprint) float64 {
	if len(f.Components) == 0 && len(other.Components) == 0 {
		return 1.0
	}

	allKeys := make(map[string]bool)
	for k := range f.Components {
		allKeys[k] = true
	}
	for k := range other.Components {
		allKeys[k] = true
	}

	if len(allKeys) == 0 {
		return 1.0
	}

	matches := 0
	for k := range allKeys {
		if f.Components[k] == other.Components[k] {
			matches++
		}
	}

	return float64(matches) / float64(len(allKeys))
}

func computeHash(components map[string]string) string {
	// Sort keys for deterministic hashing.
	keys := make([]string, 0, len(components))
	for k := range components {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte("="))
		h.Write([]byte(components[k]))
		h.Write([]byte("|"))
	}

	return hex.EncodeToString(h.Sum(nil))
}

func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	// Strip port from RemoteAddr.
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}

// IsSuspicious checks common bot indicators in the request.
func IsSuspicious(r *http.Request) (bool, []string) {
	var reasons []string

	ua := r.Header.Get("User-Agent")
	if ua == "" {
		reasons = append(reasons, "missing User-Agent")
	}

	lower := strings.ToLower(ua)
	botKeywords := []string{"bot", "crawler", "spider", "scraper", "curl", "wget", "python-requests", "go-http-client"}
	for _, kw := range botKeywords {
		if strings.Contains(lower, kw) {
			reasons = append(reasons, "bot keyword in User-Agent: "+kw)
			break
		}
	}

	if r.Header.Get("Accept-Language") == "" && r.Header.Get("Accept") == "" {
		reasons = append(reasons, "missing Accept-Language and Accept headers")
	}

	return len(reasons) > 0, reasons
}
