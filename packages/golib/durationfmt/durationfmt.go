// Package durationfmt provides human-readable duration formatting
// and parsing. Converts between time.Duration and strings like
// "2h30m", "45s", "3d12h".
package durationfmt

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Format returns a human-readable duration string.
// Uses the largest units first: days, hours, minutes, seconds.
func Format(d time.Duration) string {
	if d == 0 {
		return "0s"
	}

	neg := d < 0
	if neg {
		d = -d
	}

	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	if seconds > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%ds", seconds))
	}

	result := strings.Join(parts, "")
	if neg {
		return "-" + result
	}
	return result
}

// FormatShort returns a compact duration like "2h" or "45m".
// Uses only the most significant unit.
func FormatShort(d time.Duration) string {
	if d < 0 {
		d = -d
	}

	switch {
	case d >= 24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours())/24)
	case d >= time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d >= time.Minute:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d >= time.Second:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d >= time.Millisecond:
		return fmt.Sprintf("%dms", d.Milliseconds())
	default:
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
}

// Parse converts a human-readable duration string to time.Duration.
// Supports: Nd (days), Nh (hours), Nm (minutes), Ns (seconds), Nms (milliseconds).
func Parse(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("durationfmt: empty string")
	}

	neg := false
	if s[0] == '-' {
		neg = true
		s = s[1:]
	}

	var total time.Duration
	remaining := s

	for len(remaining) > 0 {
		// Find the next number
		i := 0
		for i < len(remaining) && remaining[i] >= '0' && remaining[i] <= '9' {
			i++
		}
		if i == 0 {
			return 0, fmt.Errorf("durationfmt: expected number at %q", remaining)
		}

		num, err := strconv.ParseInt(remaining[:i], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("durationfmt: invalid number %q", remaining[:i])
		}
		remaining = remaining[i:]

		// Find the unit
		j := 0
		for j < len(remaining) && remaining[j] >= 'a' && remaining[j] <= 'z' {
			j++
		}
		// Also handle µ
		if j < len(remaining) && remaining[j] == 194 { // µ UTF-8 first byte
			j += 2 // µ is 2 bytes
			if j < len(remaining) && remaining[j] == 's' {
				j++
			}
		}
		if j == 0 {
			return 0, fmt.Errorf("durationfmt: expected unit at %q", remaining)
		}

		unit := remaining[:j]
		remaining = remaining[j:]

		switch unit {
		case "d":
			total += time.Duration(num) * 24 * time.Hour
		case "h":
			total += time.Duration(num) * time.Hour
		case "m":
			total += time.Duration(num) * time.Minute
		case "s":
			total += time.Duration(num) * time.Second
		case "ms":
			total += time.Duration(num) * time.Millisecond
		default:
			return 0, fmt.Errorf("durationfmt: unknown unit %q", unit)
		}
	}

	if neg {
		total = -total
	}
	return total, nil
}
