package database

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var migrationFileRegex = regexp.MustCompile(`^(\d{3})_([a-z0-9_]+)\.sql$`)

// Migration represents a single SQL migration file.
type Migration struct {
	Version  string // "001", "002", etc.
	Name     string // "create_users"
	Filename string // "001_create_users.sql"
	SQL      string // file contents
}

// ParseMigrationFiles reads and sorts migration files from a directory.
func ParseMigrationFiles(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("migrate: read directory %s: %w", dir, err)
	}

	var migrations []Migration
	seen := make(map[string]string) // version -> filename

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()

		// Skip non-.sql files
		if !strings.HasSuffix(name, ".sql") {
			continue
		}

		matches := migrationFileRegex.FindStringSubmatch(name)
		if matches == nil {
			return nil, fmt.Errorf("migrate: invalid migration filename %q (expected NNN_description.sql)", name)
		}

		version := matches[1]
		desc := matches[2]

		if prev, ok := seen[version]; ok {
			return nil, fmt.Errorf("migrate: duplicate migration version %s: %s and %s", version, prev, name)
		}
		seen[version] = name

		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("migrate: read file %s: %w", name, err)
		}

		migrations = append(migrations, Migration{
			Version:  version,
			Name:     desc,
			Filename: name,
			SQL:      string(content),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// Migrate runs SQL migration files from the given directory in order.
// It creates a schema_migrations table to track applied migrations.
// Files must be named NNN_description.sql and are applied in lexicographic order.
func Migrate(db *sql.DB, migrationsDir string) error {
	// Create schema_migrations table if not exists.
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(10) PRIMARY KEY,
			name    VARCHAR(255) NOT NULL,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("migrate: create migrations table: %w", err)
	}

	migrations, err := ParseMigrationFiles(migrationsDir)
	if err != nil {
		return err
	}

	for _, m := range migrations {
		// Check if already applied.
		var exists bool
		err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", m.Version).Scan(&exists)
		if err != nil {
			return fmt.Errorf("migrate: check version %s: %w", m.Version, err)
		}
		if exists {
			slog.Debug("migration already applied", "version", m.Version, "name", m.Name)
			continue
		}

		slog.Info("applying migration", "version", m.Version, "name", m.Name)
		if _, err := db.Exec(m.SQL); err != nil {
			return fmt.Errorf("migrate: apply %s (%s): %w", m.Version, m.Name, err)
		}

		if _, err := db.Exec("INSERT INTO schema_migrations (version, name) VALUES ($1, $2)", m.Version, m.Name); err != nil {
			return fmt.Errorf("migrate: record %s: %w", m.Version, err)
		}
	}

	return nil
}
