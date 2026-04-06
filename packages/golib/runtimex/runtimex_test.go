package runtimex

import (
	"runtime"
	"testing"
)

func TestGetInfo(t *testing.T) {
	info := GetInfo()
	if info.GoVersion == "" {
		t.Error("GoVersion should not be empty")
	}
	if info.NumCPU <= 0 {
		t.Errorf("NumCPU = %d", info.NumCPU)
	}
	if info.NumGoroutine <= 0 {
		t.Errorf("NumGoroutine = %d", info.NumGoroutine)
	}
	if info.GOOS == "" {
		t.Error("GOOS should not be empty")
	}
	if info.GOARCH == "" {
		t.Error("GOARCH should not be empty")
	}
}

func TestGetMemInfo(t *testing.T) {
	info := GetMemInfo()
	if info.SysMB <= 0 {
		t.Errorf("SysMB = %f", info.SysMB)
	}
	// AllocMB can be small but should be >= 0
	if info.AllocMB < 0 {
		t.Errorf("AllocMB = %f", info.AllocMB)
	}
}

func TestGetBuildInfo(t *testing.T) {
	info := GetBuildInfo()
	// In test context, build info may or may not be available
	// Just ensure it doesn't panic
	_ = info
}

func TestNumGoroutine(t *testing.T) {
	n := NumGoroutine()
	if n <= 0 {
		t.Errorf("NumGoroutine = %d", n)
	}
}

func TestNumCPU(t *testing.T) {
	n := NumCPU()
	if n != runtime.NumCPU() {
		t.Errorf("NumCPU = %d, want %d", n, runtime.NumCPU())
	}
}
