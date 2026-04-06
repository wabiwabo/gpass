package rateheader

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSetHeaders(t *testing.T) {
	w := httptest.NewRecorder()
	reset := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)

	SetHeaders(w, Info{
		Limit:     100,
		Remaining: 42,
		Reset:     reset,
	})

	if w.Header().Get("RateLimit-Limit") != "100" {
		t.Errorf("Limit: got %q", w.Header().Get("RateLimit-Limit"))
	}
	if w.Header().Get("RateLimit-Remaining") != "42" {
		t.Errorf("Remaining: got %q", w.Header().Get("RateLimit-Remaining"))
	}
	if w.Header().Get("Retry-After") != "" {
		t.Error("Retry-After should not be set when not limited")
	}
}

func TestSetHeaders_WithRetryAfter(t *testing.T) {
	w := httptest.NewRecorder()
	SetHeaders(w, Info{
		Limit:      100,
		Remaining:  0,
		Reset:      time.Now().Add(30 * time.Second),
		RetryAfter: 30,
	})

	if w.Header().Get("Retry-After") != "30" {
		t.Errorf("Retry-After: got %q", w.Header().Get("Retry-After"))
	}
}

func TestParseHeaders(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{
			"Ratelimit-Limit":     {"100"},
			"Ratelimit-Remaining": {"42"},
			"Ratelimit-Reset":     {"1775649600"},
			"Retry-After":         {"30"},
		},
	}

	info, err := ParseHeaders(resp)
	if err != nil {
		t.Fatal(err)
	}
	if info.Limit != 100 {
		t.Errorf("Limit: got %d", info.Limit)
	}
	if info.Remaining != 42 {
		t.Errorf("Remaining: got %d", info.Remaining)
	}
	if info.RetryAfter != 30 {
		t.Errorf("RetryAfter: got %d", info.RetryAfter)
	}
}

func TestParseHeaders_Empty(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	info, err := ParseHeaders(resp)
	if err != nil {
		t.Fatal(err)
	}
	if info.Limit != 0 {
		t.Error("empty headers should give zero values")
	}
}

func TestParseHeaders_Invalid(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{
			"Ratelimit-Limit": {"not-a-number"},
		},
	}
	_, err := ParseHeaders(resp)
	if err == nil {
		t.Error("should fail on invalid number")
	}
}

func TestShouldWait_HasRemaining(t *testing.T) {
	wait := ShouldWait(Info{Remaining: 10})
	if wait != 0 {
		t.Errorf("should not wait when remaining > 0: %v", wait)
	}
}

func TestShouldWait_RetryAfter(t *testing.T) {
	wait := ShouldWait(Info{Remaining: 0, RetryAfter: 5})
	if wait != 5*time.Second {
		t.Errorf("should wait 5s: got %v", wait)
	}
}

func TestShouldWait_ResetTime(t *testing.T) {
	future := time.Now().Add(10 * time.Second)
	wait := ShouldWait(Info{Remaining: 0, Reset: future})
	if wait < 9*time.Second || wait > 10*time.Second {
		t.Errorf("should wait ~10s: got %v", wait)
	}
}

func TestShouldWait_PastReset(t *testing.T) {
	past := time.Now().Add(-time.Second)
	wait := ShouldWait(Info{Remaining: 0, Reset: past})
	if wait != 0 {
		t.Error("past reset should not wait")
	}
}

func TestSetParse_Roundtrip(t *testing.T) {
	w := httptest.NewRecorder()
	original := Info{
		Limit:      500,
		Remaining:  123,
		Reset:      time.Now().Add(time.Minute).Truncate(time.Second),
		RetryAfter: 0,
	}
	SetHeaders(w, original)

	resp := &http.Response{Header: w.Header()}
	parsed, err := ParseHeaders(resp)
	if err != nil {
		t.Fatal(err)
	}

	if parsed.Limit != original.Limit {
		t.Errorf("Limit roundtrip: %d != %d", parsed.Limit, original.Limit)
	}
	if parsed.Remaining != original.Remaining {
		t.Errorf("Remaining roundtrip: %d != %d", parsed.Remaining, original.Remaining)
	}
}
