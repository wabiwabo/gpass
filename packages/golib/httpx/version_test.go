package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
)

func TestVersionHandler_ExplicitFields(t *testing.T) {
	h := VersionHandler(VersionInfo{
		Service:   "test-svc",
		Version:   "v1.2.3",
		Commit:    "abc1234",
		BuildTime: "2026-04-07T00:00:00Z",
	})
	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
	var got VersionInfo
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Service != "test-svc" || got.Version != "v1.2.3" || got.Commit != "abc1234" {
		t.Errorf("roundtrip lost fields: %+v", got)
	}
	// GoVersion auto-filled
	if !strings.HasPrefix(got.GoVersion, "go") {
		t.Errorf("GoVersion not auto-filled: %q", got.GoVersion)
	}
}

func TestVersionHandler_AutoGoVersion(t *testing.T) {
	h := VersionHandler(VersionInfo{Service: "x"})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h(w, req)
	body := w.Body.String()
	if !strings.Contains(body, runtime.Version()) {
		t.Errorf("missing go version in body: %s", body)
	}
}

func TestVersionHandler_ContentType(t *testing.T) {
	h := VersionHandler(VersionInfo{Service: "x"})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h(w, req)
	if got := w.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q", got)
	}
}
