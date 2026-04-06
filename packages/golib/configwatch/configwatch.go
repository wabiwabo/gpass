// Package configwatch provides configuration file watching with
// automatic reload on changes. Useful for feature flags, rate limits,
// and other runtime-configurable values that shouldn't require restart.
package configwatch

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// Watcher monitors a config file and reloads on changes.
type Watcher struct {
	path     string
	interval time.Duration
	mu       sync.RWMutex
	data     map[string]interface{}
	modTime  time.Time
	running  atomic.Bool
	cancel   chan struct{}
	onChange func(map[string]interface{})
	reloads  atomic.Int64
	errors   atomic.Int64
}

// New creates a config watcher.
func New(path string, interval time.Duration) *Watcher {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return &Watcher{
		path:     path,
		interval: interval,
		data:     make(map[string]interface{}),
		cancel:   make(chan struct{}),
	}
}

// OnChange registers a callback for config changes.
func (w *Watcher) OnChange(fn func(map[string]interface{})) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onChange = fn
}

// Load reads the config file immediately.
func (w *Watcher) Load() error {
	data, modTime, err := readConfigFile(w.path)
	if err != nil {
		return err
	}

	w.mu.Lock()
	w.data = data
	w.modTime = modTime
	onChange := w.onChange
	w.mu.Unlock()

	if onChange != nil {
		onChange(data)
	}

	return nil
}

// Start begins watching the config file for changes.
func (w *Watcher) Start() error {
	if err := w.Load(); err != nil {
		return err
	}

	if w.running.Swap(true) {
		return nil // Already running.
	}

	go w.watch()
	return nil
}

// Stop stops watching.
func (w *Watcher) Stop() {
	if !w.running.Load() {
		return
	}
	close(w.cancel)
	w.running.Store(false)
}

func (w *Watcher) watch() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.cancel:
			return
		case <-ticker.C:
			w.checkReload()
		}
	}
}

func (w *Watcher) checkReload() {
	info, err := os.Stat(w.path)
	if err != nil {
		w.errors.Add(1)
		return
	}

	w.mu.RLock()
	currentMod := w.modTime
	w.mu.RUnlock()

	if !info.ModTime().After(currentMod) {
		return // No change.
	}

	data, modTime, err := readConfigFile(w.path)
	if err != nil {
		w.errors.Add(1)
		return
	}

	w.mu.Lock()
	w.data = data
	w.modTime = modTime
	onChange := w.onChange
	w.mu.Unlock()

	w.reloads.Add(1)

	if onChange != nil {
		onChange(data)
	}
}

// Get retrieves a config value.
func (w *Watcher) Get(key string) (interface{}, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	v, ok := w.data[key]
	return v, ok
}

// GetString retrieves a string config value.
func (w *Watcher) GetString(key, def string) string {
	v, ok := w.Get(key)
	if !ok {
		return def
	}
	s, ok := v.(string)
	if !ok {
		return def
	}
	return s
}

// GetBool retrieves a bool config value.
func (w *Watcher) GetBool(key string, def bool) bool {
	v, ok := w.Get(key)
	if !ok {
		return def
	}
	b, ok := v.(bool)
	if !ok {
		return def
	}
	return b
}

// GetFloat retrieves a numeric config value.
func (w *Watcher) GetFloat(key string, def float64) float64 {
	v, ok := w.Get(key)
	if !ok {
		return def
	}
	f, ok := v.(float64) // JSON numbers are float64.
	if !ok {
		return def
	}
	return f
}

// All returns a copy of all config values.
func (w *Watcher) All() map[string]interface{} {
	w.mu.RLock()
	defer w.mu.RUnlock()
	result := make(map[string]interface{}, len(w.data))
	for k, v := range w.data {
		result[k] = v
	}
	return result
}

// Stats returns reload statistics.
func (w *Watcher) Stats() (reloads, errors int64) {
	return w.reloads.Load(), w.errors.Load()
}

// IsRunning returns whether the watcher is active.
func (w *Watcher) IsRunning() bool {
	return w.running.Load()
}

func readConfigFile(path string) (map[string]interface{}, time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("configwatch: stat: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("configwatch: read: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, time.Time{}, fmt.Errorf("configwatch: parse: %w", err)
	}

	return result, info.ModTime(), nil
}
