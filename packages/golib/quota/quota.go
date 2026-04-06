package quota

import (
	"math"
	"sync"
	"time"
)

// QuotaPeriod defines the quota reset period.
type QuotaPeriod int

const (
	Daily   QuotaPeriod = iota
	Weekly
	Monthly
)

// QuotaConfig defines quota limits per tier.
type QuotaConfig struct {
	Tier   string
	Period QuotaPeriod
	Limit  int64 // 0 means unlimited
}

// Manager tracks API usage against quotas.
type Manager struct {
	configs map[string]QuotaConfig // tier -> config
	usage   map[string]*quotaUsage // key -> usage
	mu      sync.RWMutex
}

type quotaUsage struct {
	count       int64
	periodStart time.Time
}

// New creates a new quota Manager with the given configs.
func New(configs ...QuotaConfig) *Manager {
	m := &Manager{
		configs: make(map[string]QuotaConfig, len(configs)),
		usage:   make(map[string]*quotaUsage),
	}
	for _, c := range configs {
		m.configs[c.Tier] = c
	}
	return m
}

// Check checks if usage is within quota. Returns (allowed, remaining, resetAt).
// Does not consume quota.
func (m *Manager) Check(key, tier string) (bool, int64, time.Time) {
	config, ok := m.configs[tier]
	if !ok {
		return false, 0, time.Time{}
	}

	// Unlimited tier.
	if config.Limit == 0 {
		return true, math.MaxInt64, periodEnd(time.Now(), config.Period)
	}

	m.mu.RLock()
	u, exists := m.usage[key]
	m.mu.RUnlock()

	now := time.Now()
	resetAt := periodEnd(now, config.Period)

	if !exists {
		return true, config.Limit, resetAt
	}

	// Check if period has rolled over.
	if isPeriodExpired(u.periodStart, now, config.Period) {
		return true, config.Limit, resetAt
	}

	remaining := config.Limit - u.count
	if remaining < 0 {
		remaining = 0
	}
	return remaining > 0, remaining, resetAt
}

// Increment records API usage. Returns (allowed, remaining, resetAt).
func (m *Manager) Increment(key, tier string) (bool, int64, time.Time) {
	config, ok := m.configs[tier]
	if !ok {
		return false, 0, time.Time{}
	}

	// Unlimited tier.
	if config.Limit == 0 {
		m.mu.Lock()
		u, exists := m.usage[key]
		now := time.Now()
		if !exists {
			u = &quotaUsage{periodStart: periodStart(now, config.Period)}
			m.usage[key] = u
		}
		if isPeriodExpired(u.periodStart, now, config.Period) {
			u.count = 0
			u.periodStart = periodStart(now, config.Period)
		}
		u.count++
		m.mu.Unlock()
		return true, math.MaxInt64, periodEnd(now, config.Period)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	u, exists := m.usage[key]
	if !exists {
		u = &quotaUsage{periodStart: periodStart(now, config.Period)}
		m.usage[key] = u
	}

	// Reset if period expired.
	if isPeriodExpired(u.periodStart, now, config.Period) {
		u.count = 0
		u.periodStart = periodStart(now, config.Period)
	}

	resetAt := periodEnd(now, config.Period)

	if u.count >= config.Limit {
		return false, 0, resetAt
	}

	u.count++
	remaining := config.Limit - u.count
	return true, remaining, resetAt
}

// Usage returns current usage for a key.
func (m *Manager) Usage(key string) int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	u, exists := m.usage[key]
	if !exists {
		return 0
	}
	return u.count
}

// Reset resets quota for a key.
func (m *Manager) Reset(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.usage, key)
}

// DefaultQuotas returns standard GarudaPass quota configs.
func DefaultQuotas() []QuotaConfig {
	return []QuotaConfig{
		{Tier: "free", Period: Monthly, Limit: 3_000},
		{Tier: "starter", Period: Monthly, Limit: 100_000},
		{Tier: "growth", Period: Monthly, Limit: 1_000_000},
		{Tier: "enterprise", Period: Monthly, Limit: 0}, // unlimited
	}
}

// periodStart returns the start of the current period.
func periodStart(now time.Time, period QuotaPeriod) time.Time {
	switch period {
	case Daily:
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case Weekly:
		// Start of week (Monday).
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		d := now.AddDate(0, 0, -(weekday - 1))
		return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location())
	case Monthly:
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	default:
		return now
	}
}

// periodEnd returns the end of the current period (start of next period).
func periodEnd(now time.Time, period QuotaPeriod) time.Time {
	switch period {
	case Daily:
		return periodStart(now, Daily).AddDate(0, 0, 1)
	case Weekly:
		return periodStart(now, Weekly).AddDate(0, 0, 7)
	case Monthly:
		return periodStart(now, Monthly).AddDate(0, 1, 0)
	default:
		return now
	}
}

// isPeriodExpired checks if the usage period has rolled over.
func isPeriodExpired(start, now time.Time, period QuotaPeriod) bool {
	end := periodEnd(start, period)
	return now.Equal(end) || now.After(end)
}
