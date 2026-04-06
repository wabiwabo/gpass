package serviceinfo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNew(t *testing.T) {
	s := New("identity", "1.0.0")
	info := s.Get()

	if info.Name != "identity" {
		t.Errorf("name: got %q", info.Name)
	}
	if info.Version != "1.0.0" {
		t.Errorf("version: got %q", info.Version)
	}
	if info.GoVersion == "" {
		t.Error("go version should be set")
	}
	if info.Uptime == "" {
		t.Error("uptime should be set")
	}
	if info.NumCPU < 1 {
		t.Error("cpu count should be positive")
	}
}

func TestSetBuild(t *testing.T) {
	s := New("svc", "1.0")
	s.SetBuild("abc123", "2024-01-01")

	info := s.Get()
	if info.Commit != "abc123" {
		t.Errorf("commit: got %q", info.Commit)
	}
	if info.BuildTime != "2024-01-01" {
		t.Errorf("build time: got %q", info.BuildTime)
	}
}

func TestSetEnvironment(t *testing.T) {
	s := New("svc", "1.0")
	s.SetEnvironment("production")

	if s.Get().Environment != "production" {
		t.Error("environment should be set")
	}
}

func TestSetMeta(t *testing.T) {
	s := New("svc", "1.0")
	s.SetMeta("region", "ap-southeast-1")
	s.SetMeta("cluster", "eks-prod")

	info := s.Get()
	if info.Meta["region"] != "ap-southeast-1" {
		t.Errorf("region: got %q", info.Meta["region"])
	}
}

func TestHandler(t *testing.T) {
	s := New("api", "2.0.0")
	req := httptest.NewRequest(http.MethodGet, "/info", nil)
	w := httptest.NewRecorder()
	s.Handler()(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Error("should be JSON")
	}

	var info Info
	json.NewDecoder(w.Body).Decode(&info)
	if info.Name != "api" {
		t.Errorf("name: got %q", info.Name)
	}
}

func TestMetaIsolation(t *testing.T) {
	s := New("svc", "1.0")
	s.SetMeta("key", "value")

	info := s.Get()
	info.Meta["key"] = "mutated"

	// Original should not be affected.
	if s.Get().Meta["key"] != "value" {
		t.Error("Get() should return a copy")
	}
}
