// Package urlx provides URL utility functions for safe URL
// manipulation, validation, and construction.
package urlx

import (
	"net/url"
	"strings"
)

// Join safely joins a base URL with path segments.
func Join(base string, segments ...string) string {
	base = strings.TrimRight(base, "/")
	for _, seg := range segments {
		seg = strings.Trim(seg, "/")
		if seg != "" {
			base += "/" + seg
		}
	}
	return base
}

// AddQuery adds query parameters to a URL.
func AddQuery(rawURL string, params map[string]string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// IsHTTPS checks if a URL uses HTTPS.
func IsHTTPS(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return u.Scheme == "https"
}

// Host extracts the host from a URL.
func Host(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Host
}

// Path extracts the path from a URL.
func Path(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Path
}

// IsValid checks if a string is a valid URL.
func IsValid(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return u.Scheme != "" && u.Host != ""
}

// StripQuery removes query string and fragment from a URL.
func StripQuery(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

// SameOrigin checks if two URLs have the same origin (scheme + host).
func SameOrigin(a, b string) bool {
	ua, err := url.Parse(a)
	if err != nil {
		return false
	}
	ub, err := url.Parse(b)
	if err != nil {
		return false
	}
	return ua.Scheme == ub.Scheme && ua.Host == ub.Host
}
