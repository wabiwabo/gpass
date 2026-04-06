package middleware

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestFreeTier_Allow100Deny101(t *testing.T) {
	// Free tier: 100/day, 10/burst. We advance time past burst windows
	// to avoid burst limiting while testing daily limit.
	tiers := map[string]TierConfig{
		"free": {Name: "Free", DailyLimit: 100, BurstLimit: 10},
	}
	rl := NewTieredRateLimiter(tiers)

	currentTime := time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC)
	rl.now = func() time.Time { return currentTime }

	for i := 0; i < 100; i++ {
		// Advance past burst window every 10 requests.
		if i > 0 && i%10 == 0 {
			currentTime = currentTime.Add(61 * time.Second)
		}
		allowed, _, _ := rl.Allow("key1", "free")
		if !allowed {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	// Advance past burst window one more time.
	currentTime = currentTime.Add(61 * time.Second)

	allowed, remaining, _ := rl.Allow("key1", "free")
	if allowed {
		t.Fatal("request 101 should be denied")
	}
	if remaining != 0 {
		t.Errorf("remaining = %d, want 0", remaining)
	}
}

func TestStarterTier_AllowUpTo10000(t *testing.T) {
	rl := NewTieredRateLimiter(DefaultTiers())
	// Override burst to avoid burst limit blocking during rapid iteration.
	rl.tiers["starter"] = TierConfig{Name: "Starter", DailyLimit: 10000, BurstLimit: 0}

	for i := 0; i < 10000; i++ {
		allowed, _, _ := rl.Allow("key1", "starter")
		if !allowed {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	allowed, _, _ := rl.Allow("key1", "starter")
	if allowed {
		t.Fatal("request 10001 should be denied")
	}
}

func TestEnterpriseTier_AlwaysAllowed(t *testing.T) {
	rl := NewTieredRateLimiter(DefaultTiers())

	for i := 0; i < 1000; i++ {
		allowed, _, _ := rl.Allow("key1", "enterprise")
		if !allowed {
			t.Fatalf("enterprise request %d should be allowed", i+1)
		}
	}
}

func TestBurstLimit_DenyOverBurst(t *testing.T) {
	tiers := map[string]TierConfig{
		"test": {Name: "Test", DailyLimit: 1000, BurstLimit: 5},
	}
	rl := NewTieredRateLimiter(tiers)

	for i := 0; i < 5; i++ {
		allowed, _, _ := rl.Allow("key1", "test")
		if !allowed {
			t.Fatalf("burst request %d should be allowed", i+1)
		}
	}

	allowed, _, resetAt := rl.Allow("key1", "test")
	if allowed {
		t.Fatal("request 6 should be denied (burst limit)")
	}
	if resetAt.IsZero() {
		t.Error("resetAt should not be zero when burst limited")
	}
}

func TestDailyCounter_ResetsOnNewDay(t *testing.T) {
	tiers := map[string]TierConfig{
		"test": {Name: "Test", DailyLimit: 5, BurstLimit: 0},
	}
	rl := NewTieredRateLimiter(tiers)

	currentTime := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	rl.now = func() time.Time { return currentTime }

	// Use all 5 requests.
	for i := 0; i < 5; i++ {
		allowed, _, _ := rl.Allow("key1", "test")
		if !allowed {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	// Should be denied.
	allowed, _, _ := rl.Allow("key1", "test")
	if allowed {
		t.Fatal("request 6 should be denied")
	}

	// Advance to next day.
	currentTime = time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)

	// Should be allowed again.
	allowed, remaining, _ := rl.Allow("key1", "test")
	if !allowed {
		t.Fatal("request on new day should be allowed")
	}
	if remaining != 4 {
		t.Errorf("remaining = %d, want 4", remaining)
	}
}

func TestBurstCounter_ResetsAfterMinute(t *testing.T) {
	tiers := map[string]TierConfig{
		"test": {Name: "Test", DailyLimit: 1000, BurstLimit: 3},
	}
	rl := NewTieredRateLimiter(tiers)

	currentTime := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	rl.now = func() time.Time { return currentTime }

	// Use all 3 burst.
	for i := 0; i < 3; i++ {
		allowed, _, _ := rl.Allow("key1", "test")
		if !allowed {
			t.Fatalf("burst request %d should be allowed", i+1)
		}
	}

	// Should be denied.
	allowed, _, _ := rl.Allow("key1", "test")
	if allowed {
		t.Fatal("request 4 should be denied (burst)")
	}

	// Advance past the burst window.
	currentTime = currentTime.Add(61 * time.Second)

	// Should be allowed again.
	allowed, _, _ = rl.Allow("key1", "test")
	if !allowed {
		t.Fatal("request after burst window should be allowed")
	}
}

func TestUnknownTier_DefaultsToFree(t *testing.T) {
	rl := NewTieredRateLimiter(DefaultTiers())

	// Unknown tier should use free limits (100/day, 10/burst).
	for i := 0; i < 10; i++ {
		allowed, _, _ := rl.Allow("key1", "unknown_tier")
		if !allowed {
			t.Fatalf("request %d with unknown tier should be allowed (free burst=10)", i+1)
		}
	}

	// 11th should be denied by burst limit.
	allowed, _, _ := rl.Allow("key1", "unknown_tier")
	if allowed {
		t.Fatal("request 11 with unknown tier should be denied (free burst=10)")
	}
}

func TestUsage_ReturnsCorrectCount(t *testing.T) {
	rl := NewTieredRateLimiter(DefaultTiers())

	if usage := rl.Usage("key1"); usage != 0 {
		t.Errorf("initial usage = %d, want 0", usage)
	}

	for i := 0; i < 5; i++ {
		rl.Allow("key1", "free")
	}

	if usage := rl.Usage("key1"); usage != 5 {
		t.Errorf("usage after 5 requests = %d, want 5", usage)
	}
}

func TestUsage_ReturnsZeroOnNewDay(t *testing.T) {
	tiers := map[string]TierConfig{
		"test": {Name: "Test", DailyLimit: 100, BurstLimit: 0},
	}
	rl := NewTieredRateLimiter(tiers)

	currentTime := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	rl.now = func() time.Time { return currentTime }

	rl.Allow("key1", "test")
	rl.Allow("key1", "test")

	// Advance to next day.
	currentTime = time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)

	if usage := rl.Usage("key1"); usage != 0 {
		t.Errorf("usage on new day = %d, want 0", usage)
	}
}

func TestReset_ClearsCounters(t *testing.T) {
	rl := NewTieredRateLimiter(DefaultTiers())

	for i := 0; i < 5; i++ {
		rl.Allow("key1", "free")
	}

	if usage := rl.Usage("key1"); usage != 5 {
		t.Fatalf("usage before reset = %d, want 5", usage)
	}

	rl.Reset("key1")

	if usage := rl.Usage("key1"); usage != 0 {
		t.Errorf("usage after reset = %d, want 0", usage)
	}

	// Should be able to make requests again.
	allowed, _, _ := rl.Allow("key1", "free")
	if !allowed {
		t.Error("request after reset should be allowed")
	}
}

func TestConcurrentAccess(t *testing.T) {
	rl := NewTieredRateLimiter(DefaultTiers())

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", n%5)
			for j := 0; j < 20; j++ {
				rl.Allow(key, "free")
				rl.Usage(key)
			}
		}(i)
	}
	wg.Wait()

	// If we get here without a race condition panic, the test passes.
	// Run with -race to verify.
}

func TestDefaultTiers_ContainsExpectedTiers(t *testing.T) {
	tiers := DefaultTiers()

	expected := []struct {
		name       string
		dailyLimit int
		burstLimit int
	}{
		{"free", 100, 10},
		{"starter", 10000, 100},
		{"growth", 100000, 1000},
		{"enterprise", 0, 0},
	}

	for _, e := range expected {
		tc, ok := tiers[e.name]
		if !ok {
			t.Errorf("missing tier %q", e.name)
			continue
		}
		if tc.DailyLimit != e.dailyLimit {
			t.Errorf("tier %q DailyLimit = %d, want %d", e.name, tc.DailyLimit, e.dailyLimit)
		}
		if tc.BurstLimit != e.burstLimit {
			t.Errorf("tier %q BurstLimit = %d, want %d", e.name, tc.BurstLimit, e.burstLimit)
		}
	}
}
