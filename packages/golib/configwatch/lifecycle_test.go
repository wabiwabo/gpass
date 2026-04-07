package configwatch

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// TestWatcher_StartStopReload exercises the previously-uncovered hot path:
// Start → background watch goroutine → mtime change → checkReload →
// onChange callback → Stop. This is what configwatch is for in production
// (live feature flags) and was 0% covered.
func TestWatcher_StartStopReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "flags.json")
	writeConfig(t, path, `{"flag_a": true}`)

	// Backdate the initial file so the post-write mtime is unambiguously
	// newer regardless of filesystem mtime granularity.
	old := time.Now().Add(-1 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}

	w := New(path, 20*time.Millisecond)

	var fired atomic.Int32
	w.OnChange(func(m map[string]interface{}) { fired.Add(1) })

	if err := w.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(w.Stop)

	if !w.IsRunning() {
		t.Error("IsRunning should be true after Start")
	}

	// Second Start should be a no-op (running.Swap returned true).
	if err := w.Start(); err != nil {
		t.Errorf("second Start: %v", err)
	}

	// Rewrite with new content; os.WriteFile sets mtime to now, which is
	// guaranteed > old (1h ago), so checkReload will detect the change.
	writeConfig(t, path, `{"flag_a": false, "flag_b": "x"}`)

	// Note: Load() also fires onChange once during Start, so the fired
	// counter is unreliable as a "reload happened" signal — wait for the
	// new field (flag_b) to appear instead.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if w.GetString("flag_b", "") == "x" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if w.GetString("flag_b", "") != "x" {
		t.Fatalf("data not refreshed (reloads=%d errors=%d fired=%d): %v",
			w.reloads.Load(), w.errors.Load(), fired.Load(), w.All())
	}
	if fired.Load() < 2 {
		t.Errorf("OnChange should have fired at least twice (Load + reload), got %d", fired.Load())
	}

	w.Stop()
	if w.IsRunning() {
		t.Error("IsRunning should be false after Stop")
	}
	// Second Stop must be safe.
	w.Stop()
}

// TestWatcher_CheckReloadStatErrorCountsError covers checkReload's error
// branch when the underlying file disappears between ticks.
func TestWatcher_CheckReloadStatErrorCountsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vanish.json")
	writeConfig(t, path, `{"k": 1}`)

	w := New(path, time.Second)
	if err := w.Load(); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	w.checkReload() // file gone → errors++
	if _, errs := w.Stats(); errs == 0 {
		t.Error("expected error counter to increment after stat failure")
	}
}

// TestWatcher_StartLoadFailure covers Start's early-return when the initial
// Load fails (file does not exist).
func TestWatcher_StartLoadFailure(t *testing.T) {
	w := New("/nonexistent/path/that/does/not/exist.json", 10*time.Millisecond)
	if err := w.Start(); err == nil {
		t.Error("Start should fail when initial Load fails")
	}
	if w.IsRunning() {
		t.Error("IsRunning should be false when Start failed")
	}
}
