package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/garudapass/gpass/services/garudasign/signing"
)

// PostgresRequestStore is a PostgreSQL-backed RequestStore.
type PostgresRequestStore struct {
	db *sql.DB
}

func NewPostgresRequestStore(db *sql.DB) *PostgresRequestStore {
	return &PostgresRequestStore{db: db}
}

func (s *PostgresRequestStore) Create(req *signing.SigningRequest) (*signing.SigningRequest, error) {
	if err := ValidateSigningRequest(req); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Now().UTC()
	req.CreatedAt = now
	req.UpdatedAt = now

	err := s.db.QueryRowContext(ctx, `
		INSERT INTO signing_requests (
			user_id, certificate_id, document_name, document_size, document_hash,
			document_path, status, expires_at, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id`,
		req.UserID, nullStrSign(req.CertificateID), req.DocumentName, req.DocumentSize,
		req.DocumentHash, req.DocumentPath, req.Status, req.ExpiresAt,
		req.CreatedAt, req.UpdatedAt,
	).Scan(&req.ID)
	if err != nil {
		return nil, fmt.Errorf("create signing request: %w", err)
	}
	return req, nil
}

func (s *PostgresRequestStore) GetByID(id string) (*signing.SigningRequest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := s.db.QueryRowContext(ctx, requestSelectQuery+" WHERE id = $1", id)
	r, err := scanRequest(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("request not found: %s", id)
	}
	return r, err
}

func (s *PostgresRequestStore) ListByUser(userID string) ([]*signing.SigningRequest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := s.db.QueryContext(ctx,
		requestSelectQuery+" WHERE user_id = $1 ORDER BY created_at DESC", userID)
	if err != nil {
		return nil, fmt.Errorf("list requests: %w", err)
	}
	defer rows.Close()

	var result []*signing.SigningRequest
	for rows.Next() {
		r, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *PostgresRequestStore) UpdateStatus(id, status, certificateID, errorMsg string) error {
	if err := ValidateUpdateRequestStatus(status, errorMsg); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := s.db.ExecContext(ctx, `
		UPDATE signing_requests
		SET status = $1,
			certificate_id = COALESCE($2, certificate_id),
			error_message = $3,
			updated_at = NOW()
		WHERE id = $4`,
		status, nullStrSign(certificateID), nullStrSign(errorMsg), id)
	if err != nil {
		return fmt.Errorf("update request status: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("request not found: %s", id)
	}
	return nil
}

const requestSelectQuery = `
	SELECT id, user_id, COALESCE(certificate_id::text,''), document_name, document_size,
		document_hash, document_path, status, COALESCE(error_message,''),
		expires_at, created_at, updated_at
	FROM signing_requests`

func scanRequest(s rowScanner) (*signing.SigningRequest, error) {
	var r signing.SigningRequest
	if err := s.Scan(
		&r.ID, &r.UserID, &r.CertificateID, &r.DocumentName, &r.DocumentSize,
		&r.DocumentHash, &r.DocumentPath, &r.Status, &r.ErrorMessage,
		&r.ExpiresAt, &r.CreatedAt, &r.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &r, nil
}
