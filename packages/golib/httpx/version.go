package httpx

import (
	"encoding/json"
	"net/http"
	"runtime"
	"runtime/debug"
)

// VersionInfo describes a build for the /version endpoint. Service is the
// only required field; the rest are populated from runtime/debug.BuildInfo
// when zero.
type VersionInfo struct {
	Service   string `json:"service"`
	Version   string `json:"version,omitempty"`    // semver tag, if any
	Commit    string `json:"commit,omitempty"`     // git SHA
	BuildTime string `json:"build_time,omitempty"` // RFC3339
	GoVersion string `json:"go_version,omitempty"`
	Module    string `json:"module,omitempty"`
}

// VersionHandler returns an http.HandlerFunc that emits VersionInfo as JSON.
// Empty fields are auto-filled from runtime/debug.BuildInfo, which Go
// populates from VCS metadata at build time (no -ldflags required as of
// Go 1.18+).
//
// Typical usage:
//
//	mux.HandleFunc("GET /version", httpx.VersionHandler(httpx.VersionInfo{
//	    Service: "garudaaudit",
//	}))
//
// The endpoint is intentionally unauthenticated — version info is public
// in container images and Helm charts anyway. Hiding it would be theater.
func VersionHandler(info VersionInfo) http.HandlerFunc {
	// Resolve once at construction; fields don't change for the lifetime
	// of the process so there's no benefit to per-request computation.
	if info.GoVersion == "" {
		info.GoVersion = runtime.Version()
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		if info.Module == "" {
			info.Module = bi.Main.Path
		}
		if info.Version == "" && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			info.Version = bi.Main.Version
		}
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				if info.Commit == "" {
					info.Commit = s.Value
				}
			case "vcs.time":
				if info.BuildTime == "" {
					info.BuildTime = s.Value
				}
			}
		}
	}
	body, _ := json.Marshal(info)

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}
}
