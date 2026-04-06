package rfc3339

import (
	"strings"
	"testing"
	"time"
)

func TestNow(t *testing.T) {
	s := Now()
	if !strings.Contains(s, "T") {
		t.Errorf("Now() should contain T: %s", s)
	}
	if !strings.HasSuffix(s, "Z") {
		t.Errorf("Now() should end with Z (UTC): %s", s)
	}
	_, err := Parse(s)
	if err != nil {
		t.Errorf("Now() should produce parseable time: %v", err)
	}
}

func TestNowNano(t *testing.T) {
	s := NowNano()
	if !strings.Contains(s, "T") {
		t.Errorf("NowNano() should contain T: %s", s)
	}
}

func TestFormatTime(t *testing.T) {
	ts := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	got := FormatTime(ts)
	want := "2024-06-15T10:30:00Z"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatTimeNano(t *testing.T) {
	ts := time.Date(2024, 6, 15, 10, 30, 0, 123456789, time.UTC)
	got := FormatTimeNano(ts)
	if !strings.Contains(got, ".123456789") {
		t.Errorf("should contain nanoseconds: %s", got)
	}
}

func TestFormatTimeConvertsToUTC(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	ts := time.Date(2024, 6, 15, 17, 30, 0, 0, loc) // WIB = UTC+7
	got := FormatTime(ts)
	if !strings.HasSuffix(got, "Z") {
		t.Errorf("should be UTC: %s", got)
	}
	if !strings.Contains(got, "10:30") {
		t.Errorf("should be converted to UTC (10:30): %s", got)
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid", "2024-06-15T10:30:00Z", false},
		{"with_offset", "2024-06-15T10:30:00+07:00", false},
		{"invalid", "not-a-date", true},
		{"empty", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.input)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestParseNano(t *testing.T) {
	ts, err := ParseNano("2024-06-15T10:30:00.123456789Z")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if ts.Nanosecond() != 123456789 {
		t.Errorf("nanoseconds: got %d, want 123456789", ts.Nanosecond())
	}
}

func TestMustParse(t *testing.T) {
	ts := MustParse("2024-01-01T00:00:00Z")
	if ts.Year() != 2024 {
		t.Errorf("year: got %d", ts.Year())
	}
}

func TestMustParsePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	MustParse("invalid")
}

func TestIsValid(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"rfc3339", "2024-06-15T10:30:00Z", true},
		{"rfc3339_nano", "2024-06-15T10:30:00.123Z", true},
		{"offset", "2024-06-15T10:30:00+07:00", true},
		{"invalid", "2024-13-45", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValid(tt.input); got != tt.want {
				t.Errorf("IsValid(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestToUnix(t *testing.T) {
	unix, err := ToUnix("2024-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	want := int64(1704067200)
	if unix != want {
		t.Errorf("got %d, want %d", unix, want)
	}
}

func TestToUnixNano(t *testing.T) {
	unix, err := ToUnix("2024-01-01T00:00:00.123Z")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if unix != 1704067200 {
		t.Errorf("got %d", unix)
	}
}

func TestToUnixInvalid(t *testing.T) {
	_, err := ToUnix("invalid")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFromUnix(t *testing.T) {
	got := FromUnix(1704067200)
	want := "2024-01-01T00:00:00Z"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDuration(t *testing.T) {
	d, err := Duration("2024-01-01T00:00:00Z", "2024-01-01T01:00:00Z")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if d != time.Hour {
		t.Errorf("got %v, want 1h", d)
	}
}

func TestDurationInvalid(t *testing.T) {
	_, err := Duration("invalid", "2024-01-01T00:00:00Z")
	if err == nil {
		t.Fatal("expected error for invalid start")
	}
	_, err = Duration("2024-01-01T00:00:00Z", "invalid")
	if err == nil {
		t.Fatal("expected error for invalid end")
	}
}

func TestRoundTrip(t *testing.T) {
	original := time.Date(2024, 6, 15, 10, 30, 45, 0, time.UTC)
	s := FormatTime(original)
	parsed, err := Parse(s)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if !parsed.Equal(original) {
		t.Errorf("round trip mismatch: %v != %v", parsed, original)
	}
}
