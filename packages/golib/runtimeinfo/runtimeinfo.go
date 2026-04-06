// Package runtimeinfo provides Go runtime information for
// health checks and diagnostic endpoints. Exposes goroutine
// count, memory stats, and build info.
package runtimeinfo

import (
	"runtime"
	"runtime/debug"
	"time"
)

// Info holds runtime diagnostic information.
type Info struct {
	GoVersion    string `json:"go_version"`
	GOOS         string `json:"goos"`
	GOARCH       string `json:"goarch"`
	NumCPU       int    `json:"num_cpu"`
	NumGoroutine int    `json:"num_goroutine"`
	Uptime       string `json:"uptime"`
}

var startTime = time.Now()

// Get returns current runtime information.
func Get() Info {
	return Info{
		GoVersion:    runtime.Version(),
		GOOS:         runtime.GOOS,
		GOARCH:       runtime.GOARCH,
		NumCPU:       runtime.NumCPU(),
		NumGoroutine: runtime.NumGoroutine(),
		Uptime:       time.Since(startTime).Round(time.Second).String(),
	}
}

// MemStats holds simplified memory statistics.
type MemStats struct {
	Alloc      uint64 `json:"alloc_bytes"`
	TotalAlloc uint64 `json:"total_alloc_bytes"`
	Sys        uint64 `json:"sys_bytes"`
	NumGC      uint32 `json:"num_gc"`
	HeapInuse  uint64 `json:"heap_inuse_bytes"`
}

// GetMemStats returns current memory statistics.
func GetMemStats() MemStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return MemStats{
		Alloc:      m.Alloc,
		TotalAlloc: m.TotalAlloc,
		Sys:        m.Sys,
		NumGC:      m.NumGC,
		HeapInuse:  m.HeapInuse,
	}
}

// BuildInfo holds build metadata.
type BuildInfo struct {
	Main     string            `json:"main"`
	Version  string            `json:"version"`
	Settings map[string]string `json:"settings,omitempty"`
}

// GetBuildInfo returns build information from the binary.
func GetBuildInfo() BuildInfo {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return BuildInfo{}
	}
	settings := make(map[string]string)
	for _, s := range bi.Settings {
		settings[s.Key] = s.Value
	}
	return BuildInfo{
		Main:     bi.Main.Path,
		Version:  bi.Main.Version,
		Settings: settings,
	}
}

// NumGoroutine returns the current goroutine count.
func NumGoroutine() int {
	return runtime.NumGoroutine()
}

// Uptime returns the process uptime.
func Uptime() time.Duration {
	return time.Since(startTime)
}
