package quota

import (
	"testing"
	"time"
)

// TestPeriodStart_AllPeriods covers Daily/Weekly/Monthly/default branches.
// Weekly is the interesting one because GarudaPass uses Monday as the
// week start (Indonesian convention) — Sunday must wrap back to *previous*
// Monday, not skip ahead by 6 days.
func TestPeriodStart_AllPeriods(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	cases := []struct {
		name   string
		now    time.Time
		period QuotaPeriod
		want   time.Time
	}{
		{
			name:   "daily mid-day",
			now:    time.Date(2026, 4, 7, 14, 32, 11, 0, loc),
			period: Daily,
			want:   time.Date(2026, 4, 7, 0, 0, 0, 0, loc),
		},
		{
			name:   "weekly on a wednesday",
			now:    time.Date(2026, 4, 8, 12, 0, 0, 0, loc), // Wed
			period: Weekly,
			want:   time.Date(2026, 4, 6, 0, 0, 0, 0, loc),  // Mon
		},
		{
			name:   "weekly on a sunday must walk back to prior Monday",
			now:    time.Date(2026, 4, 5, 23, 59, 59, 0, loc), // Sun
			period: Weekly,
			want:   time.Date(2026, 3, 30, 0, 0, 0, 0, loc),   // Mon (6 days prior)
		},
		{
			name:   "monthly mid-month",
			now:    time.Date(2026, 4, 15, 10, 0, 0, 0, loc),
			period: Monthly,
			want:   time.Date(2026, 4, 1, 0, 0, 0, 0, loc),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := periodStart(tc.now, tc.period)
			if !got.Equal(tc.want) {
				t.Errorf("periodStart = %v, want %v", got, tc.want)
			}
		})
	}

	// Default branch: an unknown period must return `now` unchanged.
	now := time.Date(2026, 4, 7, 14, 32, 11, 0, loc)
	if got := periodStart(now, QuotaPeriod(99)); !got.Equal(now) {
		t.Errorf("default periodStart = %v, want %v (unchanged)", got, now)
	}
}

// TestPeriodEnd_AllPeriods covers the corresponding period-end branches,
// including a month rollover at year boundary (Dec → Jan).
func TestPeriodEnd_AllPeriods(t *testing.T) {
	loc := time.UTC
	cases := []struct {
		name   string
		now    time.Time
		period QuotaPeriod
		want   time.Time
	}{
		{
			name:   "daily",
			now:    time.Date(2026, 4, 7, 14, 0, 0, 0, loc),
			period: Daily,
			want:   time.Date(2026, 4, 8, 0, 0, 0, 0, loc),
		},
		{
			name:   "weekly mid-week",
			now:    time.Date(2026, 4, 8, 12, 0, 0, 0, loc), // Wed
			period: Weekly,
			want:   time.Date(2026, 4, 13, 0, 0, 0, 0, loc), // next Mon
		},
		{
			name:   "monthly",
			now:    time.Date(2026, 4, 15, 0, 0, 0, 0, loc),
			period: Monthly,
			want:   time.Date(2026, 5, 1, 0, 0, 0, 0, loc),
		},
		{
			name:   "monthly across year boundary",
			now:    time.Date(2026, 12, 25, 0, 0, 0, 0, loc),
			period: Monthly,
			want:   time.Date(2027, 1, 1, 0, 0, 0, 0, loc),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := periodEnd(tc.now, tc.period)
			if !got.Equal(tc.want) {
				t.Errorf("periodEnd = %v, want %v", got, tc.want)
			}
		})
	}

	// Default branch.
	now := time.Date(2026, 4, 7, 14, 32, 11, 0, loc)
	if got := periodEnd(now, QuotaPeriod(99)); !got.Equal(now) {
		t.Errorf("default periodEnd = %v, want %v (unchanged)", got, now)
	}
}

// TestCheck_UnlimitedTier covers Limit==0 → math.MaxInt64 path.
func TestCheck_UnlimitedTier(t *testing.T) {
	m := New(QuotaConfig{Tier: "enterprise", Period: Monthly, Limit: 0})
	allowed, remaining, reset := m.Check("k", "enterprise")
	if !allowed {
		t.Error("enterprise tier should always be allowed")
	}
	if remaining < 1<<60 {
		t.Errorf("enterprise remaining should be ~MaxInt64, got %d", remaining)
	}
	if reset.IsZero() {
		t.Error("reset should be set even for unlimited")
	}
}

// TestCheck_PeriodRollover_ResetsRemaining covers the isPeriodExpired
// branch in Check by manually backdating periodStart.
func TestCheck_PeriodRollover_ResetsRemaining(t *testing.T) {
	m := New(QuotaConfig{Tier: "free", Period: Daily, Limit: 10})
	m.Increment("k", "free") // bumps count to 1

	// Backdate periodStart to two days ago — Check must report a fresh budget.
	m.mu.Lock()
	m.usage["k"].periodStart = time.Now().Add(-48 * time.Hour)
	m.mu.Unlock()

	allowed, remaining, _ := m.Check("k", "free")
	if !allowed || remaining != 10 {
		t.Errorf("after rollover: allowed=%v remaining=%d, want true/10", allowed, remaining)
	}
}
