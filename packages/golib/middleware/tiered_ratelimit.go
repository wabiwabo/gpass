package middleware

import (
	"sync"
	"time"
)

// TierConfig defines rate limits per tier.
type TierConfig struct {
	Name       string
	DailyLimit int
	BurstLimit int // per-minute burst
}

// DefaultTiers returns the standard GarudaPass tier configuration.
func DefaultTiers() map[string]TierConfig {
	return map[string]TierConfig{
		"free":       {Name: "Free", DailyLimit: 100, BurstLimit: 10},
		"starter":    {Name: "Starter", DailyLimit: 10000, BurstLimit: 100},
		"growth":     {Name: "Growth", DailyLimit: 100000, BurstLimit: 1000},
		"enterprise": {Name: "Enterprise", DailyLimit: 0, BurstLimit: 0}, // 0 = unlimited
	}
}

// TieredRateLimiter tracks per-key usage against tier limits.
type TieredRateLimiter struct {
	tiers map[string]TierConfig
	daily map[string]*dailyCounter
	burst map[string]*burstCounter
	mu    sync.RWMutex
	now   func() time.Time // for testing
}

type dailyCounter struct {
	count int
	date  string // YYYY-MM-DD
}

type burstCounter struct {
	count       int
	windowStart time.Time
}

// NewTieredRateLimiter creates a new tiered rate limiter with the given tier configuration.
func NewTieredRateLimiter(tiers map[string]TierConfig) *TieredRateLimiter {
	return &TieredRateLimiter{
		tiers: tiers,
		daily: make(map[string]*dailyCounter),
		burst: make(map[string]*burstCounter),
		now:   time.Now,
	}
}

// Allow checks if a request is allowed for the given key and tier.
// Returns (allowed bool, remaining int, resetAt time.Time).
// Unknown tiers default to "free".
func (l *TieredRateLimiter) Allow(key, tier string) (bool, int, time.Time) {
	config, ok := l.tiers[tier]
	if !ok {
		config = l.tiers["free"]
	}

	now := l.now()
	today := now.UTC().Format("2006-01-02")

	// Enterprise tier (0 = unlimited) — always allow.
	if config.DailyLimit == 0 && config.BurstLimit == 0 {
		return true, 0, time.Time{}
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Check burst limit (per-minute window).
	if config.BurstLimit > 0 {
		bc, ok := l.burst[key]
		if !ok {
			bc = &burstCounter{windowStart: now}
			l.burst[key] = bc
		}

		// Reset burst window if more than 1 minute has passed.
		if now.Sub(bc.windowStart) >= time.Minute {
			bc.count = 0
			bc.windowStart = now
		}

		if bc.count >= config.BurstLimit {
			resetAt := bc.windowStart.Add(time.Minute)
			return false, 0, resetAt
		}
	}

	// Check daily limit.
	dc, ok := l.daily[key]
	if !ok {
		dc = &dailyCounter{date: today}
		l.daily[key] = dc
	}

	// Reset daily counter on new day.
	if dc.date != today {
		dc.count = 0
		dc.date = today
	}

	if dc.count >= config.DailyLimit {
		// Reset at midnight UTC.
		tomorrow := now.UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
		return false, 0, tomorrow
	}

	// Allow the request — increment both counters.
	dc.count++
	remaining := config.DailyLimit - dc.count

	if config.BurstLimit > 0 {
		l.burst[key].count++
	}

	tomorrow := now.UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
	return true, remaining, tomorrow
}

// Usage returns current daily usage for a key.
func (l *TieredRateLimiter) Usage(key string) int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	dc, ok := l.daily[key]
	if !ok {
		return 0
	}

	today := l.now().UTC().Format("2006-01-02")
	if dc.date != today {
		return 0
	}
	return dc.count
}

// Reset resets all counters for a key.
func (l *TieredRateLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.daily, key)
	delete(l.burst, key)
}
