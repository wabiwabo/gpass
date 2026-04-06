// Package bytefmt provides human-readable byte size formatting
// and parsing. Converts between int64 byte counts and strings
// like "1.5 GB", "256 KB", "4 MiB".
package bytefmt

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	_  = iota
	KB = 1000
	MB = 1000 * KB
	GB = 1000 * MB
	TB = 1000 * GB
	PB = 1000 * TB

	KiB = 1024
	MiB = 1024 * KiB
	GiB = 1024 * MiB
	TiB = 1024 * GiB
	PiB = 1024 * TiB
)

// Format returns a human-readable byte string using SI units (KB, MB, GB).
func Format(bytes int64) string {
	return formatUnits(bytes, 1000, []string{"B", "KB", "MB", "GB", "TB", "PB"})
}

// FormatIEC returns a human-readable byte string using IEC units (KiB, MiB, GiB).
func FormatIEC(bytes int64) string {
	return formatUnits(bytes, 1024, []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB"})
}

func formatUnits(bytes int64, base float64, units []string) string {
	if bytes == 0 {
		return "0 B"
	}

	neg := bytes < 0
	if neg {
		bytes = -bytes
	}

	b := float64(bytes)
	for i := len(units) - 1; i >= 1; i-- {
		threshold := math.Pow(base, float64(i))
		if b >= threshold {
			val := b / threshold
			s := formatFloat(val) + " " + units[i]
			if neg {
				return "-" + s
			}
			return s
		}
	}

	s := fmt.Sprintf("%d B", bytes)
	if neg {
		return "-" + s
	}
	return s
}

func formatFloat(f float64) string {
	if f == math.Trunc(f) {
		return strconv.FormatFloat(f, 'f', 0, 64)
	}
	if f*10 == math.Trunc(f*10) {
		return strconv.FormatFloat(f, 'f', 1, 64)
	}
	return strconv.FormatFloat(f, 'f', 2, 64)
}

// Parse converts a human-readable byte string to bytes.
// Supports: B, KB, MB, GB, TB, PB, KiB, MiB, GiB, TiB, PiB.
func Parse(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("bytefmt: empty string")
	}

	// Find where the number ends
	i := 0
	for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == '.' || s[i] == '-') {
		i++
	}

	numStr := strings.TrimSpace(s[:i])
	unitStr := strings.TrimSpace(s[i:])

	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("bytefmt: invalid number %q", numStr)
	}

	var multiplier float64
	switch strings.ToUpper(unitStr) {
	case "B", "":
		multiplier = 1
	case "KB":
		multiplier = float64(KB)
	case "MB":
		multiplier = float64(MB)
	case "GB":
		multiplier = float64(GB)
	case "TB":
		multiplier = float64(TB)
	case "PB":
		multiplier = float64(PB)
	case "KIB":
		multiplier = float64(KiB)
	case "MIB":
		multiplier = float64(MiB)
	case "GIB":
		multiplier = float64(GiB)
	case "TIB":
		multiplier = float64(TiB)
	case "PIB":
		multiplier = float64(PiB)
	default:
		return 0, fmt.Errorf("bytefmt: unknown unit %q", unitStr)
	}

	return int64(num * multiplier), nil
}
