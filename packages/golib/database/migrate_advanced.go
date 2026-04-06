package database

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"
)

// MigrationStatus represents the current state of a migration.
type MigrationStatus struct {
	Version   string     `json:"version"`
	Name      string     `json:"name"`
	Checksum  string     `json:"checksum"`
	State     string     `json:"state"` // "applied", "pending", "modified"
	AppliedAt *time.Time `json:"applied_at,omitempty"`
}

// MigrationPlan represents a dry-run migration plan.
type MigrationPlan struct {
	Pending  []Migration      `json:"pending"`
	Applied  []MigrationStatus `json:"applied"`
	Modified []MigrationStatus `json:"modified"` // checksum mismatch
}

// MigrateAdvanced provides enterprise migration features:
// checksum verification, dry-run mode, and status reporting.
type MigrateAdvanced struct {
	db  *sql.DB
	dir string
}

// NewMigrateAdvanced creates an advanced migration runner.
func NewMigrateAdvanced(db *sql.DB, dir string) *MigrateAdvanced {
	return &MigrateAdvanced{db: db, dir: dir}
}

// EnsureTable creates the schema_migrations table with checksum tracking.
func (m *MigrateAdvanced) EnsureTable() error {
	_, err := m.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version     VARCHAR(10) PRIMARY KEY,
			name        VARCHAR(255) NOT NULL,
			checksum    VARCHAR(64) NOT NULL DEFAULT '',
			applied_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("migrate: create table: %w", err)
	}

	// Add checksum column if it doesn't exist (upgrade path).
	_, _ = m.db.Exec(`ALTER TABLE schema_migrations ADD COLUMN IF NOT EXISTS checksum VARCHAR(64) DEFAULT ''`)
	return nil
}

// Status returns the status of all migrations.
func (m *MigrateAdvanced) Status() (*MigrationPlan, error) {
	if err := m.EnsureTable(); err != nil {
		return nil, err
	}

	migrations, err := ParseMigrationFiles(m.dir)
	if err != nil {
		return nil, err
	}

	// Get applied migrations.
	applied := make(map[string]MigrationStatus)
	rows, err := m.db.Query("SELECT version, name, checksum, applied_at FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("migrate: query applied: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var ms MigrationStatus
		var appliedAt time.Time
		if err := rows.Scan(&ms.Version, &ms.Name, &ms.Checksum, &appliedAt); err != nil {
			return nil, fmt.Errorf("migrate: scan: %w", err)
		}
		ms.AppliedAt = &appliedAt
		ms.State = "applied"
		applied[ms.Version] = ms
	}

	plan := &MigrationPlan{}
	for _, mig := range migrations {
		checksum := checksumSQL(mig.SQL)
		if existing, ok := applied[mig.Version]; ok {
			if existing.Checksum != "" && existing.Checksum != checksum {
				// Checksum mismatch — migration file was modified after being applied.
				existing.State = "modified"
				plan.Modified = append(plan.Modified, existing)
			} else {
				plan.Applied = append(plan.Applied, existing)
			}
		} else {
			plan.Pending = append(plan.Pending, mig)
		}
	}

	return plan, nil
}

// DryRun returns the migrations that would be applied without executing them.
func (m *MigrateAdvanced) DryRun() ([]Migration, error) {
	plan, err := m.Status()
	if err != nil {
		return nil, err
	}
	return plan.Pending, nil
}

// Apply runs all pending migrations with checksum tracking.
func (m *MigrateAdvanced) Apply() (int, error) {
	if err := m.EnsureTable(); err != nil {
		return 0, err
	}

	migrations, err := ParseMigrationFiles(m.dir)
	if err != nil {
		return 0, err
	}

	applied := 0
	for _, mig := range migrations {
		var exists bool
		err := m.db.QueryRow("SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", mig.Version).Scan(&exists)
		if err != nil {
			return applied, fmt.Errorf("migrate: check %s: %w", mig.Version, err)
		}
		if exists {
			continue
		}

		checksum := checksumSQL(mig.SQL)

		slog.Info("applying migration",
			"version", mig.Version,
			"name", mig.Name,
			"checksum", checksum[:12]+"...",
		)

		tx, err := m.db.Begin()
		if err != nil {
			return applied, fmt.Errorf("migrate: begin tx for %s: %w", mig.Version, err)
		}

		if _, err := tx.Exec(mig.SQL); err != nil {
			tx.Rollback()
			return applied, fmt.Errorf("migrate: apply %s (%s): %w", mig.Version, mig.Name, err)
		}

		if _, err := tx.Exec(
			"INSERT INTO schema_migrations (version, name, checksum) VALUES ($1, $2, $3)",
			mig.Version, mig.Name, checksum,
		); err != nil {
			tx.Rollback()
			return applied, fmt.Errorf("migrate: record %s: %w", mig.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return applied, fmt.Errorf("migrate: commit %s: %w", mig.Version, err)
		}

		applied++
	}

	return applied, nil
}

// VerifyChecksums validates that applied migration files haven't been modified.
func (m *MigrateAdvanced) VerifyChecksums() ([]MigrationStatus, error) {
	plan, err := m.Status()
	if err != nil {
		return nil, err
	}
	return plan.Modified, nil
}

// checksumSQL computes the SHA-256 checksum of SQL content.
func checksumSQL(sql string) string {
	h := sha256.Sum256([]byte(sql))
	return hex.EncodeToString(h[:])
}

// ChecksumSQL is exported for testing.
func ChecksumSQL(sql string) string {
	return checksumSQL(sql)
}
