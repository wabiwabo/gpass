package runtimeinfo

import (
	"runtime"
	"testing"
)

func TestGet(t *testing.T) {
	info := Get()
	if info.GoVersion == "" {
		t.Error("GoVersion should not be empty")
	}
	if info.GOOS == "" {
		t.Error("GOOS should not be empty")
	}
	if info.GOARCH == "" {
		t.Error("GOARCH should not be empty")
	}
	if info.NumCPU <= 0 {
		t.Errorf("NumCPU: got %d", info.NumCPU)
	}
	if info.NumGoroutine <= 0 {
		t.Errorf("NumGoroutine: got %d", info.NumGoroutine)
	}
	if info.Uptime == "" {
		t.Error("Uptime should not be empty")
	}
}

func TestGetGoVersion(t *testing.T) {
	info := Get()
	if info.GoVersion != runtime.Version() {
		t.Errorf("got %q, want %q", info.GoVersion, runtime.Version())
	}
}

func TestGetMemStats(t *testing.T) {
	stats := GetMemStats()
	if stats.Alloc == 0 {
		t.Error("Alloc should be > 0")
	}
	if stats.Sys == 0 {
		t.Error("Sys should be > 0")
	}
	if stats.HeapInuse == 0 {
		t.Error("HeapInuse should be > 0")
	}
}

func TestGetBuildInfo(t *testing.T) {
	info := GetBuildInfo()
	// In test context, main path may be empty but shouldn't panic
	_ = info.Main
	_ = info.Version
	_ = info.Settings
}

func TestNumGoroutine(t *testing.T) {
	n := NumGoroutine()
	if n <= 0 {
		t.Errorf("NumGoroutine: got %d", n)
	}
}

func TestUptime(t *testing.T) {
	d := Uptime()
	if d <= 0 {
		t.Errorf("Uptime: got %v", d)
	}
}

func TestGetConsistency(t *testing.T) {
	info1 := Get()
	info2 := Get()
	if info1.GOOS != info2.GOOS {
		t.Error("GOOS should be consistent")
	}
	if info1.GOARCH != info2.GOARCH {
		t.Error("GOARCH should be consistent")
	}
	if info1.NumCPU != info2.NumCPU {
		t.Error("NumCPU should be consistent")
	}
}
