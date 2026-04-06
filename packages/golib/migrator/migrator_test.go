package migrator

import (
	"sync"
	"testing"
	"time"
)

func TestNewTracker(t *testing.T) {
	tr := NewTracker()
	total, applied := tr.Count()
	if total != 0 || applied != 0 {
		t.Errorf("Count = (%d, %d)", total, applied)
	}
}

func TestRegister(t *testing.T) {
	tr := NewTracker()
	tr.Register("001", "create users table")
	tr.Register("002", "create sessions table")

	total, applied := tr.Count()
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if applied != 0 {
		t.Errorf("applied = %d, want 0", applied)
	}
}

func TestMarkApplied(t *testing.T) {
	tr := NewTracker()
	tr.Register("001", "create users table")

	if err := tr.MarkApplied("001", 50*time.Millisecond); err != nil {
		t.Fatalf("MarkApplied: %v", err)
	}

	if !tr.IsApplied("001") {
		t.Error("should be applied")
	}

	_, applied := tr.Count()
	if applied != 1 {
		t.Errorf("applied = %d", applied)
	}
}

func TestMarkApplied_NotRegistered(t *testing.T) {
	tr := NewTracker()
	if err := tr.MarkApplied("999", 0); err == nil {
		t.Error("should error for unregistered migration")
	}
}

func TestIsApplied(t *testing.T) {
	tr := NewTracker()
	tr.Register("001", "test")

	if tr.IsApplied("001") {
		t.Error("should not be applied yet")
	}

	tr.MarkApplied("001", 0)
	if !tr.IsApplied("001") {
		t.Error("should be applied")
	}

	if tr.IsApplied("999") {
		t.Error("unregistered should not be applied")
	}
}

func TestPending(t *testing.T) {
	tr := NewTracker()
	tr.Register("003", "third")
	tr.Register("001", "first")
	tr.Register("002", "second")

	tr.MarkApplied("001", 0)

	pending := tr.Pending()
	if len(pending) != 2 {
		t.Fatalf("pending = %d, want 2", len(pending))
	}
	// Sorted by version
	if pending[0].Version != "002" {
		t.Errorf("[0].Version = %q", pending[0].Version)
	}
	if pending[1].Version != "003" {
		t.Errorf("[1].Version = %q", pending[1].Version)
	}
}

func TestApplied(t *testing.T) {
	tr := NewTracker()
	tr.Register("001", "first")
	tr.Register("002", "second")
	tr.Register("003", "third")

	tr.MarkApplied("001", 10*time.Millisecond)
	tr.MarkApplied("003", 20*time.Millisecond)

	applied := tr.Applied()
	if len(applied) != 2 {
		t.Fatalf("applied = %d, want 2", len(applied))
	}
	if applied[0].Version != "001" {
		t.Errorf("[0] = %q", applied[0].Version)
	}
	if applied[1].Version != "003" {
		t.Errorf("[1] = %q", applied[1].Version)
	}
	if applied[0].Duration != 10*time.Millisecond {
		t.Errorf("Duration = %v", applied[0].Duration)
	}
}

func TestAll(t *testing.T) {
	tr := NewTracker()
	tr.Register("002", "second")
	tr.Register("001", "first")

	tr.MarkApplied("001", 0)

	all := tr.All()
	if len(all) != 2 {
		t.Fatalf("all = %d", len(all))
	}
	if all[0].Version != "001" || all[1].Version != "002" {
		t.Errorf("not sorted: %v", all)
	}
}

func TestSetChecksum(t *testing.T) {
	tr := NewTracker()
	tr.Register("001", "test")

	if err := tr.SetChecksum("001", "abc123"); err != nil {
		t.Fatalf("SetChecksum: %v", err)
	}
}

func TestSetChecksum_NotRegistered(t *testing.T) {
	tr := NewTracker()
	if err := tr.SetChecksum("999", "abc"); err == nil {
		t.Error("should error for unregistered")
	}
}

func TestVerifyChecksum_Match(t *testing.T) {
	tr := NewTracker()
	tr.Register("001", "test")
	tr.SetChecksum("001", "abc123")

	if err := tr.VerifyChecksum("001", "abc123"); err != nil {
		t.Errorf("should match: %v", err)
	}
}

func TestVerifyChecksum_Mismatch(t *testing.T) {
	tr := NewTracker()
	tr.Register("001", "test")
	tr.SetChecksum("001", "abc123")

	if err := tr.VerifyChecksum("001", "xyz789"); err == nil {
		t.Error("should error on mismatch")
	}
}

func TestVerifyChecksum_NoChecksum(t *testing.T) {
	tr := NewTracker()
	tr.Register("001", "test")

	// No checksum set, should pass
	if err := tr.VerifyChecksum("001", "anything"); err != nil {
		t.Errorf("no checksum set should pass: %v", err)
	}
}

func TestVerifyChecksum_NotRegistered(t *testing.T) {
	tr := NewTracker()
	if err := tr.VerifyChecksum("999", "abc"); err == nil {
		t.Error("should error for unregistered")
	}
}

func TestMigration_IsApplied(t *testing.T) {
	m1 := Migration{Version: "001"}
	if m1.IsApplied() {
		t.Error("should not be applied")
	}

	m2 := Migration{Version: "001", AppliedAt: time.Now()}
	if !m2.IsApplied() {
		t.Error("should be applied")
	}
}

func TestConcurrent(t *testing.T) {
	tr := NewTracker()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			tr.Register("001", "test")
		}()
		go func() {
			defer wg.Done()
			tr.Pending()
		}()
		go func() {
			defer wg.Done()
			tr.IsApplied("001")
		}()
	}
	wg.Wait()
}
