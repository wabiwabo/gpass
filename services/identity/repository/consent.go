package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Consent represents a consent record matching the consents table schema.
type Consent struct {
	ID              string
	UserID          string
	ClientID        string
	ClientName      string
	Purpose         string
	Fields          []byte // JSONB stored as raw bytes
	DurationSeconds int64
	GrantedAt       time.Time
	ExpiresAt       time.Time
	RevokedAt       *time.Time
	Status          string // ACTIVE, REVOKED, EXPIRED
	CreatedAt       time.Time
}

// ConsentRepository provides consent persistence operations.
type ConsentRepository interface {
	Grant(ctx context.Context, consent *Consent) error
	GetByID(ctx context.Context, id string) (*Consent, error)
	ListByUser(ctx context.Context, userID string) ([]*Consent, error)
	Revoke(ctx context.Context, id string) error
	ExpireStale(ctx context.Context) (int, error)
}

// PostgresConsentRepository implements ConsentRepository with PostgreSQL.
type PostgresConsentRepository struct {
	db *sql.DB
}

// NewPostgresConsentRepository creates a new PostgreSQL-backed consent repository.
func NewPostgresConsentRepository(db *sql.DB) *PostgresConsentRepository {
	return &PostgresConsentRepository{db: db}
}

// Grant inserts a new consent record and populates id and created_at from the DB.
func (r *PostgresConsentRepository) Grant(ctx context.Context, consent *Consent) error {
	query := `
		INSERT INTO consents (
			user_id, client_id, client_name, purpose, fields,
			duration_seconds, granted_at, expires_at, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at`

	err := r.db.QueryRowContext(ctx, query,
		consent.UserID,
		consent.ClientID,
		consent.ClientName,
		consent.Purpose,
		consent.Fields,
		consent.DurationSeconds,
		consent.GrantedAt,
		consent.ExpiresAt,
		consent.Status,
	).Scan(&consent.ID, &consent.CreatedAt)
	if err != nil {
		return fmt.Errorf("grant consent: %w", err)
	}
	return nil
}

// GetByID retrieves a consent by its UUID.
func (r *PostgresConsentRepository) GetByID(ctx context.Context, id string) (*Consent, error) {
	query := "SELECT " + consentColumns() + " FROM consents WHERE id = $1"
	var c Consent
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&c.ID, &c.UserID, &c.ClientID, &c.ClientName,
		&c.Purpose, &c.Fields, &c.DurationSeconds,
		&c.GrantedAt, &c.ExpiresAt, &c.RevokedAt,
		&c.Status, &c.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get consent: %w", err)
	}
	return &c, nil
}

// ListByUser retrieves all consents for a given user.
func (r *PostgresConsentRepository) ListByUser(ctx context.Context, userID string) ([]*Consent, error) {
	query := "SELECT " + consentColumns() + " FROM consents WHERE user_id = $1 ORDER BY created_at DESC"
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list consents: %w", err)
	}
	defer rows.Close()

	var consents []*Consent
	for rows.Next() {
		var c Consent
		if err := rows.Scan(
			&c.ID, &c.UserID, &c.ClientID, &c.ClientName,
			&c.Purpose, &c.Fields, &c.DurationSeconds,
			&c.GrantedAt, &c.ExpiresAt, &c.RevokedAt,
			&c.Status, &c.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan consent: %w", err)
		}
		consents = append(consents, &c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate consents: %w", err)
	}
	return consents, nil
}

// Revoke marks a consent as revoked.
func (r *PostgresConsentRepository) Revoke(ctx context.Context, id string) error {
	query := `UPDATE consents SET status = 'REVOKED', revoked_at = NOW() WHERE id = $1 AND status = 'ACTIVE'`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("revoke consent: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("revoke consent: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ExpireStale marks all active consents past their expiry as EXPIRED.
// Returns the number of consents expired.
func (r *PostgresConsentRepository) ExpireStale(ctx context.Context) (int, error) {
	query := `UPDATE consents SET status = 'EXPIRED' WHERE status = 'ACTIVE' AND expires_at < NOW()`
	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("expire stale consents: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("expire stale consents: %w", err)
	}
	return int(rows), nil
}

func consentColumns() string {
	return `id, user_id, client_id, client_name, purpose, fields,
		duration_seconds, granted_at, expires_at, revoked_at, status, created_at`
}
