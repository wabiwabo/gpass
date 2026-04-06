package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// APIKeyRecord represents an API key record.
type APIKeyRecord struct {
	ID          string
	AppID       string
	KeyHash     string
	KeyPrefix   string
	Name        string
	Environment string
	Status      string
	LastUsedAt  *time.Time
	RevokedAt   *time.Time
	ExpiresAt   *time.Time
	CreatedAt   time.Time
}

// APIKeyRepository provides API key persistence operations.
type APIKeyRepository interface {
	Create(ctx context.Context, key *APIKeyRecord) error
	GetByHash(ctx context.Context, keyHash string) (*APIKeyRecord, error)
	ListByApp(ctx context.Context, appID string) ([]*APIKeyRecord, error)
	Revoke(ctx context.Context, id string) error
	UpdateLastUsed(ctx context.Context, id string) error
}

// PostgresAPIKeyRepository implements APIKeyRepository with PostgreSQL.
type PostgresAPIKeyRepository struct {
	db *sql.DB
}

// NewPostgresAPIKeyRepository creates a new PostgreSQL-backed API key repository.
func NewPostgresAPIKeyRepository(db *sql.DB) *PostgresAPIKeyRepository {
	return &PostgresAPIKeyRepository{db: db}
}

// Create inserts a new API key record and populates id and created_at from the DB.
func (r *PostgresAPIKeyRepository) Create(ctx context.Context, key *APIKeyRecord) error {
	query := `
		INSERT INTO api_keys (
			app_id, key_hash, key_prefix, name, environment, status, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`

	err := r.db.QueryRowContext(ctx, query,
		key.AppID,
		key.KeyHash,
		key.KeyPrefix,
		key.Name,
		key.Environment,
		key.Status,
		key.ExpiresAt,
	).Scan(&key.ID, &key.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("api key already exists: %w", err)
		}
		return fmt.Errorf("create api key: %w", err)
	}
	return nil
}

// GetByHash retrieves an API key by its hash.
func (r *PostgresAPIKeyRepository) GetByHash(ctx context.Context, keyHash string) (*APIKeyRecord, error) {
	query := "SELECT " + apiKeyColumns() + " FROM api_keys WHERE key_hash = $1"
	var k APIKeyRecord
	err := r.db.QueryRowContext(ctx, query, keyHash).Scan(
		&k.ID, &k.AppID, &k.KeyHash, &k.KeyPrefix,
		&k.Name, &k.Environment, &k.Status,
		&k.LastUsedAt, &k.RevokedAt, &k.ExpiresAt, &k.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get api key: %w", err)
	}
	return &k, nil
}

// ListByApp retrieves all API keys for a given app.
func (r *PostgresAPIKeyRepository) ListByApp(ctx context.Context, appID string) ([]*APIKeyRecord, error) {
	query := "SELECT " + apiKeyColumns() + " FROM api_keys WHERE app_id = $1 ORDER BY created_at DESC"
	rows, err := r.db.QueryContext(ctx, query, appID)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()

	var keys []*APIKeyRecord
	for rows.Next() {
		var k APIKeyRecord
		if err := rows.Scan(
			&k.ID, &k.AppID, &k.KeyHash, &k.KeyPrefix,
			&k.Name, &k.Environment, &k.Status,
			&k.LastUsedAt, &k.RevokedAt, &k.ExpiresAt, &k.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		keys = append(keys, &k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api keys: %w", err)
	}
	return keys, nil
}

// Revoke marks an API key as revoked.
func (r *PostgresAPIKeyRepository) Revoke(ctx context.Context, id string) error {
	query := `UPDATE api_keys SET status = 'revoked', revoked_at = NOW() WHERE id = $1 AND status = 'active'`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateLastUsed updates the last_used_at timestamp.
func (r *PostgresAPIKeyRepository) UpdateLastUsed(ctx context.Context, id string) error {
	query := `UPDATE api_keys SET last_used_at = NOW() WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("update last used: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update last used: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func apiKeyColumns() string {
	return `id, app_id, key_hash, key_prefix, name, environment, status,
		last_used_at, revoked_at, expires_at, created_at`
}
