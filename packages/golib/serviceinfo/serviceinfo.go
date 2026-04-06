// Package serviceinfo provides runtime service information for
// debugging, monitoring, and status endpoints. Collects build info,
// dependency versions, uptime, and runtime stats.
package serviceinfo

import (
	"encoding/json"
	"net/http"
	"runtime"
	"sync"
	"time"
)

// Info holds service runtime information.
type Info struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Commit      string            `json:"commit,omitempty"`
	BuildTime   string            `json:"build_time,omitempty"`
	GoVersion   string            `json:"go_version"`
	OS          string            `json:"os"`
	Arch        string            `json:"arch"`
	NumCPU      int               `json:"num_cpu"`
	Goroutines  int               `json:"goroutines"`
	StartedAt   time.Time         `json:"started_at"`
	Uptime      string            `json:"uptime"`
	Environment string            `json:"environment,omitempty"`
	Meta        map[string]string `json:"meta,omitempty"`
}

// Service manages service information.
type Service struct {
	mu        sync.RWMutex
	name      string
	version   string
	commit    string
	buildTime string
	env       string
	startedAt time.Time
	meta      map[string]string
}

// New creates a service info instance.
func New(name, version string) *Service {
	return &Service{
		name:      name,
		version:   version,
		startedAt: time.Now(),
		meta:      make(map[string]string),
	}
}

// SetBuild sets build-time information.
func (s *Service) SetBuild(commit, buildTime string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commit = commit
	s.buildTime = buildTime
}

// SetEnvironment sets the deployment environment.
func (s *Service) SetEnvironment(env string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.env = env
}

// SetMeta sets a metadata key-value pair.
func (s *Service) SetMeta(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.meta[key] = value
}

// Get returns the current service info snapshot.
func (s *Service) Get() Info {
	s.mu.RLock()
	defer s.mu.RUnlock()

	meta := make(map[string]string, len(s.meta))
	for k, v := range s.meta {
		meta[k] = v
	}

	return Info{
		Name:        s.name,
		Version:     s.version,
		Commit:      s.commit,
		BuildTime:   s.buildTime,
		GoVersion:   runtime.Version(),
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		NumCPU:      runtime.NumCPU(),
		Goroutines:  runtime.NumGoroutine(),
		StartedAt:   s.startedAt,
		Uptime:      time.Since(s.startedAt).Round(time.Second).String(),
		Environment: s.env,
		Meta:        meta,
	}
}

// Handler returns an HTTP handler that serves service info as JSON.
func (s *Service) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(s.Get())
	}
}
