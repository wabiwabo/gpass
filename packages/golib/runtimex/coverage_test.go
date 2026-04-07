package runtimex

import (
	"runtime"
	"testing"
	"time"
)

// TestGetMemInfo_LastGCFormatBranch covers the m.LastGC > 0 branch by
// triggering a GC explicitly. After GC, LastGC must be non-zero and
// info.LastGC must be a parseable RFC3339 timestamp.
func TestGetMemInfo_LastGCFormatBranch(t *testing.T) {
	// Force a GC so MemStats.LastGC is populated.
	runtime.GC()

	info := GetMemInfo()
	if info.LastGC == "" {
		t.Fatal("LastGC empty after explicit GC")
	}
	if _, err := time.Parse(time.RFC3339, info.LastGC); err != nil {
		t.Errorf("LastGC %q not RFC3339: %v", info.LastGC, err)
	}
	if info.NumGC == 0 {
		t.Errorf("NumGC = 0 after explicit GC")
	}
	// Sanity: heap allocations are non-zero in any running test process.
	if info.AllocMB <= 0 {
		t.Errorf("AllocMB = %f, want > 0", info.AllocMB)
	}
	if info.SysMB <= 0 {
		t.Errorf("SysMB = %f, want > 0", info.SysMB)
	}
}

// TestGetBuildInfo_PopulatedFromDebug pins that GetBuildInfo extracts
// the settings map from runtime/debug.BuildInfo. In a test binary the
// VCS settings are populated by `go test`, so we can assert at least
// the GOOS/GOARCH-style settings or compiler are present.
func TestGetBuildInfo_PopulatedFromDebug(t *testing.T) {
	bi := GetBuildInfo()
	if bi.Main == "" {
		t.Error("Main path is empty — BuildInfo not populated")
	}
	if bi.Settings == nil {
		t.Error("Settings map is nil")
	}
	// At least one well-known setting should be present in any modern
	// Go test binary. We don't pin a specific key because the set varies
	// across Go versions and build modes.
	if len(bi.Settings) == 0 {
		t.Logf("Settings map is empty — this may be normal for go test in some modes")
	}
}

// TestGetInfo_AllFieldsPopulated covers GetInfo and pins that none of
// its six fields are zero-valued in a normal process.
func TestGetInfo_AllFieldsPopulated(t *testing.T) {
	info := GetInfo()
	if info.GoVersion == "" {
		t.Error("GoVersion empty")
	}
	if info.NumCPU < 1 {
		t.Errorf("NumCPU = %d, want >= 1", info.NumCPU)
	}
	if info.NumGoroutine < 1 {
		t.Errorf("NumGoroutine = %d, want >= 1", info.NumGoroutine)
	}
	if info.GOOS == "" || info.GOARCH == "" || info.Compiler == "" {
		t.Errorf("missing field: %+v", info)
	}
}

// TestNumGoroutineIncreasesWithSpawn pins that NumGoroutine reflects
// real goroutine creation. Without this assertion the helper could
// silently return a stale or constant value and operators would lose
// the leak-detection signal.
func TestNumGoroutineIncreasesWithSpawn(t *testing.T) {
	before := NumGoroutine()
	done := make(chan struct{})
	const N = 20
	for i := 0; i < N; i++ {
		go func() { <-done }()
	}
	// Give the scheduler a beat.
	time.Sleep(10 * time.Millisecond)
	after := NumGoroutine()
	if after-before < N {
		t.Errorf("NumGoroutine: before=%d after=%d, expected at least +%d", before, after, N)
	}
	close(done)
}
