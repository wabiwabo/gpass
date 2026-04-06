package database

import (
	"testing"
)

func TestChecksumSQL_Deterministic(t *testing.T) {
	sql := "CREATE TABLE users (id SERIAL PRIMARY KEY);"
	c1 := ChecksumSQL(sql)
	c2 := ChecksumSQL(sql)

	if c1 != c2 {
		t.Error("same SQL should produce same checksum")
	}
	if len(c1) != 64 {
		t.Errorf("checksum length: got %d, want 64 (SHA-256 hex)", len(c1))
	}
}

func TestChecksumSQL_DifferentSQL(t *testing.T) {
	c1 := ChecksumSQL("CREATE TABLE a (id INT);")
	c2 := ChecksumSQL("CREATE TABLE b (id INT);")

	if c1 == c2 {
		t.Error("different SQL should produce different checksums")
	}
}

func TestChecksumSQL_EmptySQL(t *testing.T) {
	c := ChecksumSQL("")
	if c == "" {
		t.Error("empty SQL should still produce a checksum")
	}
}

func TestMigrationStatus_Fields(t *testing.T) {
	ms := MigrationStatus{
		Version:  "001",
		Name:     "create_users",
		Checksum: "abc123",
		State:    "pending",
	}

	if ms.Version != "001" {
		t.Error("version mismatch")
	}
	if ms.State != "pending" {
		t.Error("state mismatch")
	}
	if ms.AppliedAt != nil {
		t.Error("applied_at should be nil for pending")
	}
}

func TestMigrationPlan_Empty(t *testing.T) {
	plan := &MigrationPlan{}
	if len(plan.Pending) != 0 {
		t.Error("expected no pending")
	}
	if len(plan.Applied) != 0 {
		t.Error("expected no applied")
	}
	if len(plan.Modified) != 0 {
		t.Error("expected no modified")
	}
}

func TestChecksumSQL_WhitespaceMatters(t *testing.T) {
	c1 := ChecksumSQL("SELECT 1;")
	c2 := ChecksumSQL("SELECT  1;")

	if c1 == c2 {
		t.Error("whitespace differences should produce different checksums")
	}
}
