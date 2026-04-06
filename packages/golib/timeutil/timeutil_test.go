package timeutil

import (
	"testing"
	"time"
)

func TestWIB_Offset(t *testing.T) {
	_, offset := time.Now().In(WIB).Zone()
	if offset != 7*60*60 {
		t.Errorf("WIB offset: got %d, want %d", offset, 7*60*60)
	}
}

func TestWITA_Offset(t *testing.T) {
	_, offset := time.Now().In(WITA).Zone()
	if offset != 8*60*60 {
		t.Errorf("WITA offset: got %d", offset)
	}
}

func TestWIT_Offset(t *testing.T) {
	_, offset := time.Now().In(WIT).Zone()
	if offset != 9*60*60 {
		t.Errorf("WIT offset: got %d", offset)
	}
}

func TestNowWIB(t *testing.T) {
	wib := NowWIB()
	zone, _ := wib.Zone()
	if zone != "WIB" {
		t.Errorf("zone: got %q", zone)
	}
}

func TestToWIB(t *testing.T) {
	utc := time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC)
	wib := ToWIB(utc)
	if wib.Hour() != 7 {
		t.Errorf("0:00 UTC should be 7:00 WIB, got %d", wib.Hour())
	}
}

func TestFormatIndonesian(t *testing.T) {
	dt := time.Date(2026, 4, 6, 7, 30, 0, 0, WIB)
	formatted := FormatIndonesian(dt)
	if formatted != "06 April 2026, 07:30 WIB" {
		t.Errorf("format: got %q", formatted)
	}
}

func TestFormatIndonesian_December(t *testing.T) {
	dt := time.Date(2026, 12, 25, 14, 0, 0, 0, WIB)
	formatted := FormatIndonesian(dt)
	if formatted != "25 Desember 2026, 14:00 WIB" {
		t.Errorf("format: got %q", formatted)
	}
}

func TestFormatISO(t *testing.T) {
	dt := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	iso := FormatISO(dt)
	if iso != "2026-04-06T12:00:00Z" {
		t.Errorf("ISO: got %q", iso)
	}
}

func TestStartOfDay(t *testing.T) {
	dt := time.Date(2026, 4, 6, 14, 30, 45, 123, WIB)
	start := StartOfDay(dt)
	if start.Hour() != 0 || start.Minute() != 0 || start.Second() != 0 {
		t.Errorf("start of day: got %v", start)
	}
}

func TestEndOfDay(t *testing.T) {
	dt := time.Date(2026, 4, 6, 14, 30, 0, 0, WIB)
	end := EndOfDay(dt)
	if end.Hour() != 23 || end.Minute() != 59 || end.Second() != 59 {
		t.Errorf("end of day: got %v", end)
	}
}

func TestDaysBetween(t *testing.T) {
	a := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	b := time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC)
	if DaysBetween(a, b) != 5 {
		t.Errorf("days: got %d, want 5", DaysBetween(a, b))
	}
	// Order shouldn't matter.
	if DaysBetween(b, a) != 5 {
		t.Error("reverse order should also be 5")
	}
}

func TestIsBusinessDay(t *testing.T) {
	mon := time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC) // Monday
	sat := time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC) // Saturday
	sun := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC) // Sunday

	if !IsBusinessDay(mon) {
		t.Error("Monday should be business day")
	}
	if IsBusinessDay(sat) {
		t.Error("Saturday should not be business day")
	}
	if IsBusinessDay(sun) {
		t.Error("Sunday should not be business day")
	}
}

func TestParseFlexible(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"2026-04-06T12:00:00Z", true},
		{"2026-04-06T12:00:00", true},
		{"2026-04-06 12:00:00", true},
		{"2026-04-06", true},
		{"06/04/2026", true},
		{"not a date", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := ParseFlexible(tt.input)
			if (err == nil) != tt.valid {
				t.Errorf("ParseFlexible(%q): err=%v, want valid=%v", tt.input, err, tt.valid)
			}
		})
	}
}
