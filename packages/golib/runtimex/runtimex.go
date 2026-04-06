// Package runtimex provides runtime information utilities for
// service introspection. Reports Go version, goroutine count,
// memory stats, and build info for health endpoints.
package runtimex

import (
	"runtime"
	"runtime/debug"
	"time"
)

// Info contains runtime information for a service.
type Info struct {
	GoVersion    string `json:"go_version"`
	NumCPU       int    `json:"num_cpu"`
	NumGoroutine int    `json:"num_goroutine"`
	GOOS         string `json:"goos"`
	GOARCH       string `json:"goarch"`
	Compiler     string `json:"compiler"`
}

// GetInfo returns current runtime information.
func GetInfo() Info {
	return Info{
		GoVersion:    runtime.Version(),
		NumCPU:       runtime.NumCPU(),
		NumGoroutine: runtime.NumGoroutine(),
		GOOS:         runtime.GOOS,
		GOARCH:       runtime.GOARCH,
		Compiler:     runtime.Compiler,
	}
}

// MemInfo contains memory statistics.
type MemInfo struct {
	AllocMB      float64 `json:"alloc_mb"`
	TotalAllocMB float64 `json:"total_alloc_mb"`
	SysMB        float64 `json:"sys_mb"`
	NumGC        uint32  `json:"num_gc"`
	LastGC       string  `json:"last_gc,omitempty"`
}

// GetMemInfo returns current memory statistics.
func GetMemInfo() MemInfo {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	info := MemInfo{
		AllocMB:      float64(m.Alloc) / 1024 / 1024,
		TotalAllocMB: float64(m.TotalAlloc) / 1024 / 1024,
		SysMB:        float64(m.Sys) / 1024 / 1024,
		NumGC:        m.NumGC,
	}

	if m.LastGC > 0 {
		info.LastGC = time.Unix(0, int64(m.LastGC)).UTC().Format(time.RFC3339)
	}

	return info
}

// BuildInfo contains build metadata.
type BuildInfo struct {
	Main    string            `json:"main,omitempty"`
	Version string            `json:"version,omitempty"`
	Sum     string            `json:"sum,omitempty"`
	Settings map[string]string `json:"settings,omitempty"`
}

// GetBuildInfo returns build information from runtime/debug.
func GetBuildInfo() BuildInfo {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return BuildInfo{}
	}

	bi := BuildInfo{
		Main:     info.Main.Path,
		Version:  info.Main.Version,
		Sum:      info.Main.Sum,
		Settings: make(map[string]string),
	}

	for _, s := range info.Settings {
		bi.Settings[s.Key] = s.Value
	}

	return bi
}

// NumGoroutine returns the current number of goroutines.
func NumGoroutine() int {
	return runtime.NumGoroutine()
}

// NumCPU returns the number of logical CPUs.
func NumCPU() int {
	return runtime.NumCPU()
}
