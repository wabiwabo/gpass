package rateheader

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// Info holds rate limit information for response headers.
type Info struct {
	// Limit is the maximum number of requests in the window.
	Limit int
	// Remaining is requests remaining in the current window.
	Remaining int
	// Reset is when the current window resets (Unix timestamp).
	Reset time.Time
	// RetryAfter is seconds until the client should retry (only set when limited).
	RetryAfter int
}

// SetHeaders writes rate limit information into HTTP response headers.
// Uses the IETF draft standard headers: RateLimit-Limit, RateLimit-Remaining, RateLimit-Reset.
func SetHeaders(w http.ResponseWriter, info Info) {
	w.Header().Set("RateLimit-Limit", strconv.Itoa(info.Limit))
	w.Header().Set("RateLimit-Remaining", strconv.Itoa(info.Remaining))
	w.Header().Set("RateLimit-Reset", strconv.FormatInt(info.Reset.Unix(), 10))

	if info.RetryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(info.RetryAfter))
	}
}

// ParseHeaders reads rate limit information from HTTP response headers.
func ParseHeaders(resp *http.Response) (Info, error) {
	info := Info{}

	if v := resp.Header.Get("RateLimit-Limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return info, fmt.Errorf("parse RateLimit-Limit: %w", err)
		}
		info.Limit = n
	}

	if v := resp.Header.Get("RateLimit-Remaining"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return info, fmt.Errorf("parse RateLimit-Remaining: %w", err)
		}
		info.Remaining = n
	}

	if v := resp.Header.Get("RateLimit-Reset"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return info, fmt.Errorf("parse RateLimit-Reset: %w", err)
		}
		info.Reset = time.Unix(n, 0)
	}

	if v := resp.Header.Get("Retry-After"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return info, fmt.Errorf("parse Retry-After: %w", err)
		}
		info.RetryAfter = n
	}

	return info, nil
}

// ShouldWait returns the duration to wait before retrying based on rate limit headers.
// Returns 0 if no waiting is needed.
func ShouldWait(info Info) time.Duration {
	if info.Remaining > 0 {
		return 0
	}
	if info.RetryAfter > 0 {
		return time.Duration(info.RetryAfter) * time.Second
	}
	wait := time.Until(info.Reset)
	if wait > 0 {
		return wait
	}
	return 0
}
