package store

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"
)

// Stores bundles all garudacorp store implementations together.
type Stores struct {
	Entity EntityStore
	Role   RoleStore
	UBO    UBOStore
	DB     *sql.DB // nil for in-memory mode
}

// NewStoresFromEnv returns Postgres-backed stores if GARUDACORP_DB_URL or
// DATABASE_URL is set, otherwise in-memory fallback for development/testing.
// Enforces 12factor Factor VI in production.
func NewStoresFromEnv() (*Stores, error) {
	dsn := os.Getenv("GARUDACORP_DB_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		return &Stores{
			Entity: NewInMemoryEntityStore(),
			Role:   NewInMemoryRoleStore(),
			UBO:    NewInMemoryUBOStore(),
		}, nil
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	configurePool(db)
	return &Stores{
		Entity: NewPostgresEntityStore(db),
		Role:   NewPostgresRoleStore(db),
		UBO:    NewPostgresUBOStore(db),
		DB:     db,
	}, nil
}
