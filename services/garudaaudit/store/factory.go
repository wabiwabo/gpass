package store

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"
)

// New returns a Postgres-backed AuditStore if DATABASE_URL is set, otherwise
// falls back to InMemoryAuditStore for tests/dev. This selection enforces
// 12factor Factor VI (stateless processes) in production.
func New(db *sql.DB) AuditStore {
	if db != nil {
		return NewPostgresAuditStore(db)
	}
	return NewInMemoryAuditStore()
}

// NewFromEnv opens a Postgres connection using GARUDAAUDIT_DB_URL or
// DATABASE_URL and returns a Postgres-backed store. Returns InMemoryAuditStore
// if neither env var is set (development/testing fallback).
func NewFromEnv() (AuditStore, *sql.DB, error) {
	dsn := os.Getenv("GARUDAAUDIT_DB_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		return NewInMemoryAuditStore(), nil, nil
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("ping postgres: %w", err)
	}
	configurePool(db)
	return NewPostgresAuditStore(db), db, nil
}
