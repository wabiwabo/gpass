package timeutil

import (
	"fmt"
	"time"
)

// Indonesian timezone locations.
var (
	WIB  *time.Location // UTC+7 (Western Indonesia - Jakarta)
	WITA *time.Location // UTC+8 (Central Indonesia - Makassar)
	WIT  *time.Location // UTC+9 (Eastern Indonesia - Jayapura)
)

func init() {
	WIB = time.FixedZone("WIB", 7*60*60)
	WITA = time.FixedZone("WITA", 8*60*60)
	WIT = time.FixedZone("WIT", 9*60*60)
}

// NowWIB returns the current time in WIB (Jakarta time).
func NowWIB() time.Time {
	return time.Now().In(WIB)
}

// NowWITA returns the current time in WITA (Makassar time).
func NowWITA() time.Time {
	return time.Now().In(WITA)
}

// NowWIT returns the current time in WIT (Jayapura time).
func NowWIT() time.Time {
	return time.Now().In(WIT)
}

// ToWIB converts a time to WIB timezone.
func ToWIB(t time.Time) time.Time {
	return t.In(WIB)
}

// FormatIndonesian formats a time in Indonesian format: "06 April 2026, 14:30 WIB".
func FormatIndonesian(t time.Time) string {
	wib := t.In(WIB)
	months := []string{
		"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
		"Juli", "Agustus", "September", "Oktober", "November", "Desember",
	}
	return fmt.Sprintf("%02d %s %d, %02d:%02d WIB",
		wib.Day(), months[wib.Month()], wib.Year(),
		wib.Hour(), wib.Minute())
}

// FormatISO returns time in ISO 8601 format with timezone.
func FormatISO(t time.Time) string {
	return t.Format(time.RFC3339)
}

// StartOfDay returns midnight (00:00:00) of the given day.
func StartOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// EndOfDay returns 23:59:59.999999999 of the given day.
func EndOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, t.Location())
}

// DaysBetween returns the number of days between two times.
func DaysBetween(a, b time.Time) int {
	diff := b.Sub(a)
	if diff < 0 {
		diff = -diff
	}
	return int(diff.Hours() / 24)
}

// IsBusinessDay returns true if the day is Monday-Friday.
func IsBusinessDay(t time.Time) bool {
	day := t.Weekday()
	return day != time.Saturday && day != time.Sunday
}

// ParseFlexible tries multiple common formats to parse a time string.
func ParseFlexible(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"02/01/2006",          // DD/MM/YYYY (Indonesian format)
		"02-01-2006",
		"January 2, 2006",
	}

	for _, fmt := range formats {
		if t, err := time.Parse(fmt, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("timeutil: cannot parse %q", s)
}
