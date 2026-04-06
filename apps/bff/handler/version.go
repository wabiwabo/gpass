package handler

import (
	"encoding/json"
	"net/http"
	"runtime"
)

// VersionHandler returns platform version and build information.
type VersionHandler struct {
	info VersionInfo
}

// VersionInfo contains platform metadata exposed by the version endpoint.
type VersionInfo struct {
	Platform    string `json:"platform"`
	Version     string `json:"version"`
	APIVersion  string `json:"api_version"`
	GoVersion   string `json:"go_version"`
	Commit      string `json:"commit"`
	BuildTime   string `json:"build_time"`
	Services    int    `json:"services"`
	Environment string `json:"environment"`
}

// DefaultVersionInfo creates a VersionInfo with sensible defaults.
// The platform name is always "GarudaPass" and GoVersion is read from
// the runtime. The services count reflects the current platform service
// count (12 services).
func DefaultVersionInfo(version, commit, buildTime, env string) VersionInfo {
	return VersionInfo{
		Platform:    "GarudaPass",
		Version:     version,
		APIVersion:  "v1",
		GoVersion:   runtime.Version(),
		Commit:      commit,
		BuildTime:   buildTime,
		Services:    12,
		Environment: env,
	}
}

// NewVersionHandler creates a VersionHandler with the given version info.
func NewVersionHandler(info VersionInfo) *VersionHandler {
	return &VersionHandler{info: info}
}

// ServeHTTP handles GET /api/v1/version — returns platform version and
// build metadata as JSON.
func (h *VersionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(h.info)
}
