package store

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"
)

// NewDeletionStoreFromEnv returns a Postgres-backed DeletionStore if
// IDENTITY_DB_URL or DATABASE_URL is set, otherwise InMemoryDeletionStore.
// Enforces 12factor Factor VI in production.
func NewDeletionStoreFromEnv() (DeletionStore, *sql.DB, error) {
	dsn := os.Getenv("IDENTITY_DB_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		return NewInMemoryDeletionStore(), nil, nil
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("ping postgres: %w", err)
	}
	return NewPostgresDeletionStore(db), db, nil
}
