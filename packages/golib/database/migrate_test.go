package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMigrationFilesSortsCorrectly(t *testing.T) {
	dir := t.TempDir()

	// Write files out of order.
	files := map[string]string{
		"003_add_index.sql":    "CREATE INDEX idx ON users(email);",
		"001_create_users.sql": "CREATE TABLE users (id SERIAL PRIMARY KEY);",
		"002_add_email.sql":    "ALTER TABLE users ADD COLUMN email TEXT;",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	migrations, err := ParseMigrationFiles(dir)
	if err != nil {
		t.Fatalf("ParseMigrationFiles: %v", err)
	}

	if len(migrations) != 3 {
		t.Fatalf("got %d migrations, want 3", len(migrations))
	}

	expected := []struct {
		version  string
		name     string
		filename string
	}{
		{"001", "create_users", "001_create_users.sql"},
		{"002", "add_email", "002_add_email.sql"},
		{"003", "add_index", "003_add_index.sql"},
	}

	for i, e := range expected {
		m := migrations[i]
		if m.Version != e.version {
			t.Errorf("migration[%d].Version = %q, want %q", i, m.Version, e.version)
		}
		if m.Name != e.name {
			t.Errorf("migration[%d].Name = %q, want %q", i, m.Name, e.name)
		}
		if m.Filename != e.filename {
			t.Errorf("migration[%d].Filename = %q, want %q", i, m.Filename, e.filename)
		}
		if m.SQL == "" {
			t.Errorf("migration[%d].SQL is empty", i)
		}
	}
}

func TestParseMigrationFilesSkipsNonSQL(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "001_init.sql"), []byte("CREATE TABLE t(id INT);"), 0o644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Migrations"), 0o644)
	os.WriteFile(filepath.Join(dir, ".gitkeep"), []byte(""), 0o644)

	migrations, err := ParseMigrationFiles(dir)
	if err != nil {
		t.Fatalf("ParseMigrationFiles: %v", err)
	}

	if len(migrations) != 1 {
		t.Fatalf("got %d migrations, want 1", len(migrations))
	}
	if migrations[0].Version != "001" {
		t.Errorf("Version = %q, want %q", migrations[0].Version, "001")
	}
}

func TestParseMigrationFilesDetectsDuplicateVersions(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "001_create_users.sql"), []byte("SQL1"), 0o644)
	os.WriteFile(filepath.Join(dir, "001_create_accounts.sql"), []byte("SQL2"), 0o644)

	_, err := ParseMigrationFiles(dir)
	if err == nil {
		t.Fatal("expected error for duplicate versions, got nil")
	}
	if got := err.Error(); !contains(got, "duplicate migration version") {
		t.Errorf("error = %q, want to contain 'duplicate migration version'", got)
	}
}

func TestParseMigrationFilesRejectsInvalidNames(t *testing.T) {
	tests := []struct {
		name     string
		filename string
	}{
		{"no version prefix", "create_users.sql"},
		{"wrong version format", "01_create_users.sql"},
		{"uppercase in name", "001_Create_Users.sql"},
		{"hyphens in name", "001_create-users.sql"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			os.WriteFile(filepath.Join(dir, tt.filename), []byte("SQL"), 0o644)

			_, err := ParseMigrationFiles(dir)
			if err == nil {
				t.Fatalf("expected error for filename %q, got nil", tt.filename)
			}
		})
	}
}

func TestParseMigrationFilesEmptyDir(t *testing.T) {
	dir := t.TempDir()

	migrations, err := ParseMigrationFiles(dir)
	if err != nil {
		t.Fatalf("ParseMigrationFiles: %v", err)
	}
	if len(migrations) != 0 {
		t.Errorf("got %d migrations, want 0", len(migrations))
	}
}

func TestParseMigrationFilesNonexistentDir(t *testing.T) {
	_, err := ParseMigrationFiles("/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for nonexistent directory, got nil")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
