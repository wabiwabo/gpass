// Package rfc3339 provides RFC 3339 timestamp formatting and parsing.
// Ensures consistent timestamp formats across all GarudaPass services
// for API responses, audit logs, and event timestamps.
package rfc3339

import (
	"time"
)

// Format is the RFC 3339 layout.
const Format = time.RFC3339

// FormatNano is the RFC 3339 layout with nanoseconds.
const FormatNano = time.RFC3339Nano

// Now returns the current time formatted as RFC 3339.
func Now() string {
	return time.Now().UTC().Format(Format)
}

// NowNano returns the current time formatted as RFC 3339 with nanoseconds.
func NowNano() string {
	return time.Now().UTC().Format(FormatNano)
}

// FormatTime formats a time.Time as RFC 3339 in UTC.
func FormatTime(t time.Time) string {
	return t.UTC().Format(Format)
}

// FormatTimeNano formats a time.Time as RFC 3339 with nanoseconds in UTC.
func FormatTimeNano(t time.Time) string {
	return t.UTC().Format(FormatNano)
}

// Parse parses an RFC 3339 string.
func Parse(s string) (time.Time, error) {
	return time.Parse(Format, s)
}

// ParseNano parses an RFC 3339 string with nanoseconds.
func ParseNano(s string) (time.Time, error) {
	return time.Parse(FormatNano, s)
}

// MustParse parses an RFC 3339 string, panicking on error.
func MustParse(s string) time.Time {
	t, err := Parse(s)
	if err != nil {
		panic("rfc3339: " + err.Error())
	}
	return t
}

// IsValid checks if a string is a valid RFC 3339 timestamp.
func IsValid(s string) bool {
	_, err := time.Parse(Format, s)
	if err != nil {
		_, err = time.Parse(FormatNano, s)
	}
	return err == nil
}

// ToUnix converts an RFC 3339 string to Unix timestamp.
func ToUnix(s string) (int64, error) {
	t, err := time.Parse(Format, s)
	if err != nil {
		t, err = time.Parse(FormatNano, s)
		if err != nil {
			return 0, err
		}
	}
	return t.Unix(), nil
}

// FromUnix converts a Unix timestamp to RFC 3339 string.
func FromUnix(unix int64) string {
	return time.Unix(unix, 0).UTC().Format(Format)
}

// Duration returns the duration between two RFC 3339 timestamps.
func Duration(start, end string) (time.Duration, error) {
	s, err := Parse(start)
	if err != nil {
		return 0, err
	}
	e, err := Parse(end)
	if err != nil {
		return 0, err
	}
	return e.Sub(s), nil
}
