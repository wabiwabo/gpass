// Package embedinfo provides build-time metadata injection via
// Go linker flags (-ldflags). Stores version, commit, build time,
// and environment for service identification in health endpoints.
package embedinfo

import (
	"encoding/json"
	"runtime"
)

// Values are set via -ldflags at build time:
//
//	go build -ldflags "-X embedinfo.Version=1.0.0 -X embedinfo.Commit=abc123"
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
	GoVersion = runtime.Version()
	Env       = "development"
)

// Info holds all build metadata.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
	GoVersion string `json:"go_version"`
	Env       string `json:"env"`
}

// Get returns the current build info.
func Get() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		BuildTime: BuildTime,
		GoVersion: GoVersion,
		Env:       Env,
	}
}

// JSON returns build info as JSON bytes.
func JSON() ([]byte, error) {
	return json.Marshal(Get())
}

// IsDev returns true if running in development mode.
func IsDev() bool {
	return Env == "development" || Env == "dev"
}

// IsProduction returns true if running in production.
func IsProduction() bool {
	return Env == "production" || Env == "prod"
}

// Short returns a compact version string like "1.0.0-abc123".
func Short() string {
	if Commit == "unknown" || len(Commit) < 7 {
		return Version
	}
	return Version + "-" + Commit[:7]
}
