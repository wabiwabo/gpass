// Package migrator provides a simple schema migration tracking system.
// Records which migrations have been applied and prevents re-application.
// Designed for use with database migration runners.
package migrator

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// Migration represents a single schema migration.
type Migration struct {
	Version     string    `json:"version"`
	Description string    `json:"description"`
	AppliedAt   time.Time `json:"applied_at,omitempty"`
	Duration    time.Duration `json:"duration,omitempty"`
	Checksum    string    `json:"checksum,omitempty"`
}

// IsApplied returns true if the migration has been applied.
func (m Migration) IsApplied() bool {
	return !m.AppliedAt.IsZero()
}

// Tracker manages migration state.
type Tracker struct {
	mu         sync.RWMutex
	migrations map[string]Migration
}

// NewTracker creates a migration tracker.
func NewTracker() *Tracker {
	return &Tracker{
		migrations: make(map[string]Migration),
	}
}

// Register records a migration as available.
func (t *Tracker) Register(version, description string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.migrations[version] = Migration{
		Version:     version,
		Description: description,
	}
}

// MarkApplied marks a migration as applied.
func (t *Tracker) MarkApplied(version string, duration time.Duration) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	m, ok := t.migrations[version]
	if !ok {
		return fmt.Errorf("migration %q not registered", version)
	}

	m.AppliedAt = time.Now().UTC()
	m.Duration = duration
	t.migrations[version] = m
	return nil
}

// SetChecksum sets the checksum for a migration.
func (t *Tracker) SetChecksum(version, checksum string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	m, ok := t.migrations[version]
	if !ok {
		return fmt.Errorf("migration %q not registered", version)
	}
	m.Checksum = checksum
	t.migrations[version] = m
	return nil
}

// IsApplied checks if a specific migration version has been applied.
func (t *Tracker) IsApplied(version string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	m, ok := t.migrations[version]
	return ok && m.IsApplied()
}

// Pending returns unapplied migrations sorted by version.
func (t *Tracker) Pending() []Migration {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []Migration
	for _, m := range t.migrations {
		if !m.IsApplied() {
			result = append(result, m)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Version < result[j].Version
	})
	return result
}

// Applied returns applied migrations sorted by version.
func (t *Tracker) Applied() []Migration {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []Migration
	for _, m := range t.migrations {
		if m.IsApplied() {
			result = append(result, m)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Version < result[j].Version
	})
	return result
}

// All returns all migrations sorted by version.
func (t *Tracker) All() []Migration {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]Migration, 0, len(t.migrations))
	for _, m := range t.migrations {
		result = append(result, m)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Version < result[j].Version
	})
	return result
}

// Count returns total and applied counts.
func (t *Tracker) Count() (total, applied int) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	total = len(t.migrations)
	for _, m := range t.migrations {
		if m.IsApplied() {
			applied++
		}
	}
	return
}

// VerifyChecksum checks if a migration's checksum matches.
func (t *Tracker) VerifyChecksum(version, checksum string) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	m, ok := t.migrations[version]
	if !ok {
		return fmt.Errorf("migration %q not registered", version)
	}

	if m.Checksum == "" {
		return nil // no checksum set, skip verification
	}

	if m.Checksum != checksum {
		return fmt.Errorf("migration %q checksum mismatch: expected %q, got %q",
			version, m.Checksum, checksum)
	}

	return nil
}
