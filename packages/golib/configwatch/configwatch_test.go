package configwatch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeConfig(t *testing.T, path string, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestWatcher_Load(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeConfig(t, path, `{"feature_x": true, "rate_limit": 100}`)

	w := New(path, time.Second)
	if err := w.Load(); err != nil {
		t.Fatal(err)
	}

	if w.GetBool("feature_x", false) != true {
		t.Error("feature_x should be true")
	}
	if w.GetFloat("rate_limit", 0) != 100 {
		t.Errorf("rate_limit: got %f", w.GetFloat("rate_limit", 0))
	}
}

func TestWatcher_GetString(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeConfig(t, path, `{"name": "garudapass"}`)

	w := New(path, time.Second)
	w.Load()

	if w.GetString("name", "") != "garudapass" {
		t.Error("name should be garudapass")
	}
	if w.GetString("missing", "default") != "default" {
		t.Error("missing should return default")
	}
}

func TestWatcher_GetBool_Default(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeConfig(t, path, `{}`)

	w := New(path, time.Second)
	w.Load()

	if w.GetBool("missing", true) != true {
		t.Error("missing bool should return default")
	}
}

func TestWatcher_All(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeConfig(t, path, `{"a": 1, "b": 2}`)

	w := New(path, time.Second)
	w.Load()

	all := w.All()
	if len(all) != 2 {
		t.Errorf("all: got %d", len(all))
	}
}

func TestWatcher_AllIsolation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeConfig(t, path, `{"key": "value"}`)

	w := New(path, time.Second)
	w.Load()

	all := w.All()
	all["key"] = "mutated"

	if w.GetString("key", "") != "value" {
		t.Error("All() should return a copy")
	}
}

func TestWatcher_InvalidFile(t *testing.T) {
	w := New("/nonexistent/config.json", time.Second)
	if err := w.Load(); err == nil {
		t.Error("should fail for missing file")
	}
}

func TestWatcher_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeConfig(t, path, `not json`)

	w := New(path, time.Second)
	if err := w.Load(); err == nil {
		t.Error("should fail for invalid JSON")
	}
}

func TestWatcher_OnChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeConfig(t, path, `{"v": 1}`)

	var called bool
	w := New(path, time.Second)
	w.OnChange(func(data map[string]interface{}) {
		called = true
	})
	w.Load()

	if !called {
		t.Error("OnChange should fire on Load")
	}
}

func TestWatcher_Stats(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeConfig(t, path, `{"v": 1}`)

	w := New(path, time.Second)
	w.Load()

	reloads, errors := w.Stats()
	if reloads != 0 { // Load doesn't count as reload.
		t.Errorf("reloads: got %d", reloads)
	}
	if errors != 0 {
		t.Errorf("errors: got %d", errors)
	}
}

func TestWatcher_DefaultInterval(t *testing.T) {
	w := New("/tmp/test.json", 0) // Should default.
	if w.interval != 5*time.Second {
		t.Errorf("interval: got %v", w.interval)
	}
}
