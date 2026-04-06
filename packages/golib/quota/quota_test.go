package quota

import (
	"math"
	"sync"
	"testing"
)

func TestIncrement_WithinQuota(t *testing.T) {
	m := New(QuotaConfig{Tier: "free", Period: Monthly, Limit: 3000})

	allowed, remaining, resetAt := m.Increment("app-1", "free")
	if !allowed {
		t.Error("expected allowed")
	}
	if remaining != 2999 {
		t.Errorf("expected 2999 remaining, got %d", remaining)
	}
	if resetAt.IsZero() {
		t.Error("expected non-zero resetAt")
	}
}

func TestIncrement_ExceedingQuota(t *testing.T) {
	m := New(QuotaConfig{Tier: "test", Period: Monthly, Limit: 3})

	for i := 0; i < 3; i++ {
		allowed, _, _ := m.Increment("app-1", "test")
		if !allowed {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// 4th request should fail.
	allowed, remaining, _ := m.Increment("app-1", "test")
	if allowed {
		t.Error("expected not allowed after exceeding quota")
	}
	if remaining != 0 {
		t.Errorf("expected 0 remaining, got %d", remaining)
	}
}

func TestIncrement_RemainingDecrements(t *testing.T) {
	m := New(QuotaConfig{Tier: "test", Period: Monthly, Limit: 5})

	for i := int64(0); i < 5; i++ {
		allowed, remaining, _ := m.Increment("app-1", "test")
		if !allowed {
			t.Errorf("request %d should be allowed", i+1)
		}
		expected := int64(4) - i
		if remaining != expected {
			t.Errorf("request %d: expected %d remaining, got %d", i+1, expected, remaining)
		}
	}
}

func TestIncrement_MonthlyRollover(t *testing.T) {
	m := New(QuotaConfig{Tier: "test", Period: Monthly, Limit: 2})

	// Exhaust quota.
	m.Increment("app-1", "test")
	m.Increment("app-1", "test")

	allowed, _, _ := m.Increment("app-1", "test")
	if allowed {
		t.Error("should be denied after exhausting quota")
	}

	// Simulate period rollover by manipulating the usage start time.
	m.mu.Lock()
	u := m.usage["app-1"]
	u.periodStart = u.periodStart.AddDate(0, -1, 0) // Move back one month.
	m.mu.Unlock()

	// Should be allowed again after rollover.
	allowed, remaining, _ := m.Increment("app-1", "test")
	if !allowed {
		t.Error("should be allowed after period rollover")
	}
	if remaining != 1 {
		t.Errorf("expected 1 remaining after rollover, got %d", remaining)
	}
}

func TestFreeTier_3000PerMonth(t *testing.T) {
	configs := DefaultQuotas()
	m := New(configs...)

	// Verify free tier limit.
	for i := 0; i < 3000; i++ {
		allowed, _, _ := m.Increment("free-app", "free")
		if !allowed {
			t.Fatalf("free tier request %d should be allowed", i+1)
		}
	}

	allowed, _, _ := m.Increment("free-app", "free")
	if allowed {
		t.Error("free tier should deny after 3000 requests")
	}
}

func TestEnterprise_Unlimited(t *testing.T) {
	configs := DefaultQuotas()
	m := New(configs...)

	for i := 0; i < 10000; i++ {
		allowed, remaining, _ := m.Increment("ent-app", "enterprise")
		if !allowed {
			t.Fatalf("enterprise request %d should be allowed", i+1)
		}
		if remaining != math.MaxInt64 {
			t.Fatalf("enterprise remaining should be MaxInt64, got %d", remaining)
		}
	}
}

func TestCheck_DoesNotConsumeQuota(t *testing.T) {
	m := New(QuotaConfig{Tier: "test", Period: Monthly, Limit: 5})

	// Check multiple times.
	for i := 0; i < 10; i++ {
		allowed, remaining, _ := m.Check("app-1", "test")
		if !allowed {
			t.Error("check should report allowed")
		}
		if remaining != 5 {
			t.Errorf("check should not consume quota, remaining=%d", remaining)
		}
	}

	if m.Usage("app-1") != 0 {
		t.Errorf("usage should be 0 after checks only, got %d", m.Usage("app-1"))
	}
}

func TestReset_ClearsCount(t *testing.T) {
	m := New(QuotaConfig{Tier: "test", Period: Monthly, Limit: 100})

	m.Increment("app-1", "test")
	m.Increment("app-1", "test")
	m.Increment("app-1", "test")

	if m.Usage("app-1") != 3 {
		t.Errorf("expected usage 3, got %d", m.Usage("app-1"))
	}

	m.Reset("app-1")

	if m.Usage("app-1") != 0 {
		t.Errorf("expected usage 0 after reset, got %d", m.Usage("app-1"))
	}

	// Should be able to increment again.
	allowed, remaining, _ := m.Increment("app-1", "test")
	if !allowed {
		t.Error("should be allowed after reset")
	}
	if remaining != 99 {
		t.Errorf("expected 99 remaining, got %d", remaining)
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := New(QuotaConfig{Tier: "test", Period: Monthly, Limit: 10000})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				m.Increment("app-1", "test")
			}
		}()
	}
	wg.Wait()

	usage := m.Usage("app-1")
	if usage != 10000 {
		t.Errorf("expected usage 10000 after concurrent access, got %d", usage)
	}
}

func TestCheck_UnknownTier(t *testing.T) {
	m := New(QuotaConfig{Tier: "free", Period: Monthly, Limit: 3000})

	allowed, remaining, _ := m.Check("app-1", "nonexistent")
	if allowed {
		t.Error("unknown tier should not be allowed")
	}
	if remaining != 0 {
		t.Errorf("expected 0 remaining for unknown tier, got %d", remaining)
	}
}

func TestDefaultQuotas(t *testing.T) {
	configs := DefaultQuotas()
	if len(configs) != 4 {
		t.Fatalf("expected 4 default configs, got %d", len(configs))
	}

	expected := map[string]int64{
		"free":       3_000,
		"starter":    100_000,
		"growth":     1_000_000,
		"enterprise": 0,
	}

	for _, c := range configs {
		limit, ok := expected[c.Tier]
		if !ok {
			t.Errorf("unexpected tier %q", c.Tier)
			continue
		}
		if c.Limit != limit {
			t.Errorf("tier %q: expected limit %d, got %d", c.Tier, limit, c.Limit)
		}
		if c.Period != Monthly {
			t.Errorf("tier %q: expected Monthly period", c.Tier)
		}
	}
}
