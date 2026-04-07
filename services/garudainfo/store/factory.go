package store

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"
)

// NewConsentStoreFromEnv returns a Postgres-backed ConsentStore if
// GARUDAINFO_DB_URL or DATABASE_URL is set, otherwise an InMemoryConsentStore
// (development/testing fallback). Enforces 12factor Factor VI in production.
func NewConsentStoreFromEnv() (ConsentStore, *sql.DB, error) {
	dsn := os.Getenv("GARUDAINFO_DB_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		return NewInMemoryConsentStore(), nil, nil
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("ping postgres: %w", err)
	}
	return NewPostgresConsentStore(db), db, nil
}
