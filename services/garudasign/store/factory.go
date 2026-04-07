package store

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"
)

// Stores bundles all garudasign store implementations.
type Stores struct {
	Certificate CertificateStore
	Request     RequestStore
	Document    DocumentStore
	DB          *sql.DB // nil for in-memory mode
}

// NewStoresFromEnv returns Postgres-backed stores if GARUDASIGN_DB_URL or
// DATABASE_URL is set, otherwise in-memory fallback. Enforces 12factor Factor VI.
func NewStoresFromEnv() (*Stores, error) {
	dsn := os.Getenv("GARUDASIGN_DB_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		return &Stores{
			Certificate: NewInMemoryCertificateStore(),
			Request:     NewInMemoryRequestStore(),
			Document:    NewInMemoryDocumentStore(),
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
		Certificate: NewPostgresCertificateStore(db),
		Request:     NewPostgresRequestStore(db),
		Document:    NewPostgresDocumentStore(db),
		DB:          db,
	}, nil
}
