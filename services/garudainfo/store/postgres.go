package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// PostgresConsentStore is a PostgreSQL-backed implementation of ConsentStore.
// It satisfies 12factor Factor VI (stateless processes) and UU PDP No. 27/2022
// consent persistence requirements.
type PostgresConsentStore struct {
	db *sql.DB
}

// NewPostgresConsentStore creates a new PostgreSQL-backed consent store.
func NewPostgresConsentStore(db *sql.DB) *PostgresConsentStore {
	return &PostgresConsentStore{db: db}
}

func (s *PostgresConsentStore) Create(ctx context.Context, c *Consent) error {
	if err := ValidateConsent(c); err != nil {
		return err
	}
	fieldsJSON, err := json.Marshal(c.Fields)
	if err != nil {
		return fmt.Errorf("marshal fields: %w", err)
	}

	now := time.Now().UTC()
	c.Status = "ACTIVE"
	c.GrantedAt = now
	c.ExpiresAt = now.Add(time.Duration(c.DurationSeconds) * time.Second)

	query := `
		INSERT INTO consents (
			user_id, client_id, client_name, purpose, fields,
			duration_seconds, granted_at, expires_at, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id`

	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return s.db.QueryRowContext(cctx, query,
		c.UserID, c.ClientID, c.ClientName, c.Purpose, fieldsJSON,
		c.DurationSeconds, c.GrantedAt, c.ExpiresAt, c.Status,
	).Scan(&c.ID)
}

func (s *PostgresConsentStore) GetByID(ctx context.Context, id string) (*Consent, error) {
	query := `
		SELECT id, user_id, client_id, client_name, purpose, fields,
			duration_seconds, granted_at, expires_at, revoked_at, status
		FROM consents WHERE id = $1`

	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	row := s.db.QueryRowContext(cctx, query, id)
	c, err := scanConsent(row)
	if err == sql.ErrNoRows {
		return nil, ErrConsentNotFound
	}
	return c, err
}

func (s *PostgresConsentStore) ListByUser(ctx context.Context, userID string) ([]*Consent, error) {
	query := `
		SELECT id, user_id, client_id, client_name, purpose, fields,
			duration_seconds, granted_at, expires_at, revoked_at, status
		FROM consents WHERE user_id = $1 ORDER BY granted_at DESC`

	return s.queryConsents(ctx, query, userID)
}

func (s *PostgresConsentStore) ListActiveByUserAndClient(ctx context.Context, userID, clientID string) ([]*Consent, error) {
	query := `
		SELECT id, user_id, client_id, client_name, purpose, fields,
			duration_seconds, granted_at, expires_at, revoked_at, status
		FROM consents
		WHERE user_id = $1 AND client_id = $2 AND status = 'ACTIVE'
		ORDER BY granted_at DESC`

	return s.queryConsents(ctx, query, userID, clientID)
}

func (s *PostgresConsentStore) Revoke(ctx context.Context, id string) error {
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	now := time.Now().UTC()
	res, err := s.db.ExecContext(cctx, `
		UPDATE consents
		SET status = 'REVOKED', revoked_at = $1
		WHERE id = $2 AND status != 'REVOKED'`, now, id)
	if err != nil {
		return fmt.Errorf("revoke consent: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		// Distinguish not-found from already-revoked
		var status string
		err := s.db.QueryRowContext(cctx, `SELECT status FROM consents WHERE id = $1`, id).Scan(&status)
		if err == sql.ErrNoRows {
			return ErrConsentNotFound
		}
		if err != nil {
			return err
		}
		if status == "REVOKED" {
			return ErrConsentRevoked
		}
	}
	return nil
}

func (s *PostgresConsentStore) ExpireStale(ctx context.Context) (int, error) {
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	res, err := s.db.ExecContext(cctx, `
		UPDATE consents
		SET status = 'EXPIRED'
		WHERE status = 'ACTIVE' AND expires_at < NOW()`)
	if err != nil {
		return 0, fmt.Errorf("expire stale consents: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func (s *PostgresConsentStore) queryConsents(ctx context.Context, query string, args ...interface{}) ([]*Consent, error) {
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	rows, err := s.db.QueryContext(cctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query consents: %w", err)
	}
	defer rows.Close()

	var result []*Consent
	for rows.Next() {
		c, err := scanConsent(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

type scanner interface {
	Scan(dest ...interface{}) error
}

func scanConsent(s scanner) (*Consent, error) {
	var c Consent
	var fieldsJSON []byte
	var revokedAt sql.NullTime
	if err := s.Scan(
		&c.ID, &c.UserID, &c.ClientID, &c.ClientName, &c.Purpose,
		&fieldsJSON, &c.DurationSeconds, &c.GrantedAt, &c.ExpiresAt,
		&revokedAt, &c.Status,
	); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(fieldsJSON, &c.Fields); err != nil {
		return nil, fmt.Errorf("unmarshal fields: %w", err)
	}
	if revokedAt.Valid {
		t := revokedAt.Time
		c.RevokedAt = &t
	}
	return &c, nil
}
