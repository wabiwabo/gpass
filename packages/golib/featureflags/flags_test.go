package featureflags

import (
	"sync"
	"testing"
)

func TestIsEnabled_EnabledFlag(t *testing.T) {
	s := New(Flag{Name: "feature.a", Enabled: true})
	if !s.IsEnabled("feature.a") {
		t.Fatal("expected feature.a to be enabled")
	}
}

func TestIsEnabled_DisabledFlag(t *testing.T) {
	s := New(Flag{Name: "feature.b", Enabled: false})
	if s.IsEnabled("feature.b") {
		t.Fatal("expected feature.b to be disabled")
	}
}

func TestIsEnabled_UnknownFlag_ReturnsFalse(t *testing.T) {
	s := New()
	if s.IsEnabled("nonexistent") {
		t.Fatal("expected unknown flag to return false")
	}
}

func TestIsEnabledForUser_100Percent_AlwaysTrue(t *testing.T) {
	s := New(Flag{Name: "rollout", Enabled: true, Percentage: 100})
	for i := range 100 {
		userID := "user-" + string(rune('A'+i))
		if !s.IsEnabledForUser("rollout", userID) {
			t.Fatalf("expected 100%% rollout to be true for %s", userID)
		}
	}
}

func TestIsEnabledForUser_0Percent_AlwaysFalse(t *testing.T) {
	s := New(Flag{Name: "rollout", Enabled: true, Percentage: 0})
	for i := range 100 {
		userID := "user-" + string(rune('A'+i))
		if s.IsEnabledForUser("rollout", userID) {
			t.Fatalf("expected 0%% rollout to be false for %s", userID)
		}
	}
}

func TestIsEnabledForUser_50Percent_Deterministic(t *testing.T) {
	s := New(Flag{Name: "half", Enabled: true, Percentage: 50})

	// Same user should always get the same result.
	first := s.IsEnabledForUser("half", "user-stable")
	for range 100 {
		if s.IsEnabledForUser("half", "user-stable") != first {
			t.Fatal("expected deterministic result for same user")
		}
	}
}

func TestIsEnabledForUser_50Percent_SplitsUsers(t *testing.T) {
	s := New(Flag{Name: "split", Enabled: true, Percentage: 50})

	enabled := 0
	total := 1000
	for i := range total {
		userID := "user-" + string(rune(i))
		if s.IsEnabledForUser("split", userID) {
			enabled++
		}
	}

	// With 50% and 1000 users, expect roughly 400-600.
	if enabled < 300 || enabled > 700 {
		t.Fatalf("expected roughly 50%% split, got %d/%d enabled", enabled, total)
	}
}

func TestIsEnabledForUser_DisabledFlag_AlwaysFalse(t *testing.T) {
	s := New(Flag{Name: "disabled", Enabled: false, Percentage: 100})
	if s.IsEnabledForUser("disabled", "any-user") {
		t.Fatal("expected disabled flag to return false even with 100% rollout")
	}
}

func TestSet_ToggleFlag(t *testing.T) {
	s := New(Flag{Name: "toggle", Enabled: false})

	s.Set("toggle", true)
	if !s.IsEnabled("toggle") {
		t.Fatal("expected toggle to be enabled after Set(true)")
	}

	s.Set("toggle", false)
	if s.IsEnabled("toggle") {
		t.Fatal("expected toggle to be disabled after Set(false)")
	}
}

func TestSet_CreatesNewFlag(t *testing.T) {
	s := New()
	s.Set("new-flag", true)
	if !s.IsEnabled("new-flag") {
		t.Fatal("expected new flag to be enabled")
	}
}

func TestSetPercentage(t *testing.T) {
	s := New(Flag{Name: "gradual", Enabled: true, Percentage: 0})

	s.SetPercentage("gradual", 50)
	flags := s.All()
	for _, f := range flags {
		if f.Name == "gradual" && f.Percentage != 50 {
			t.Fatalf("expected percentage 50, got %d", f.Percentage)
		}
	}
}

func TestSetPercentage_Clamps(t *testing.T) {
	s := New(Flag{Name: "clamp", Enabled: true})

	s.SetPercentage("clamp", -10)
	flags := s.All()
	for _, f := range flags {
		if f.Name == "clamp" && f.Percentage != 0 {
			t.Fatalf("expected percentage clamped to 0, got %d", f.Percentage)
		}
	}

	s.SetPercentage("clamp", 200)
	flags = s.All()
	for _, f := range flags {
		if f.Name == "clamp" && f.Percentage != 100 {
			t.Fatalf("expected percentage clamped to 100, got %d", f.Percentage)
		}
	}
}

func TestAll_ReturnsCompleteList(t *testing.T) {
	s := New(
		Flag{Name: "a", Enabled: true},
		Flag{Name: "b", Enabled: false},
		Flag{Name: "c", Enabled: true},
	)

	all := s.All()
	if len(all) != 3 {
		t.Fatalf("expected 3 flags, got %d", len(all))
	}

	names := make(map[string]bool)
	for _, f := range all {
		names[f.Name] = true
	}
	for _, expected := range []string{"a", "b", "c"} {
		if !names[expected] {
			t.Fatalf("expected flag %q in All()", expected)
		}
	}
}

func TestDefaultFlags_ExpectedCount(t *testing.T) {
	flags := DefaultFlags()
	if len(flags) != 8 {
		t.Fatalf("expected 8 default flags, got %d", len(flags))
	}
}

func TestConcurrentAccess(t *testing.T) {
	s := New(Flag{Name: "concurrent", Enabled: true, Percentage: 50})

	var wg sync.WaitGroup
	const goroutines = 100

	wg.Add(goroutines * 3)
	for range goroutines {
		go func() {
			defer wg.Done()
			s.IsEnabled("concurrent")
		}()
		go func() {
			defer wg.Done()
			s.IsEnabledForUser("concurrent", "user-1")
		}()
		go func() {
			defer wg.Done()
			s.Set("concurrent", true)
		}()
	}
	wg.Wait()
}
