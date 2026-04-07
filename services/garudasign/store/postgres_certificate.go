package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/garudapass/gpass/services/garudasign/signing"
)

// PostgresCertificateStore is a PostgreSQL-backed CertificateStore.
// 12factor Factor VI compliant.
type PostgresCertificateStore struct {
	db *sql.DB
}

func NewPostgresCertificateStore(db *sql.DB) *PostgresCertificateStore {
	return &PostgresCertificateStore{db: db}
}

func (s *PostgresCertificateStore) Create(cert *signing.Certificate) (*signing.Certificate, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Now().UTC()
	cert.CreatedAt = now
	cert.UpdatedAt = now

	err := s.db.QueryRowContext(ctx, `
		INSERT INTO signing_certificates (
			user_id, serial_number, issuer_dn, subject_dn, status,
			valid_from, valid_to, certificate_pem, fingerprint_sha256,
			created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id`,
		cert.UserID, cert.SerialNumber, cert.IssuerDN, cert.SubjectDN, cert.Status,
		cert.ValidFrom, cert.ValidTo, cert.CertificatePEM, cert.FingerprintSHA256,
		cert.CreatedAt, cert.UpdatedAt,
	).Scan(&cert.ID)
	if err != nil {
		return nil, fmt.Errorf("create certificate: %w", err)
	}
	return cert, nil
}

func (s *PostgresCertificateStore) GetByID(id string) (*signing.Certificate, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := s.db.QueryRowContext(ctx, certSelectQuery+" WHERE id = $1", id)
	cert, err := scanCertificate(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("certificate not found: %s", id)
	}
	return cert, err
}

func (s *PostgresCertificateStore) GetActiveByUser(userID string) (*signing.Certificate, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := s.db.QueryRowContext(ctx,
		certSelectQuery+" WHERE user_id = $1 AND status = 'ACTIVE' LIMIT 1", userID)
	cert, err := scanCertificate(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no active certificate found for user: %s", userID)
	}
	return cert, err
}

func (s *PostgresCertificateStore) ListByUser(userID, statusFilter string) ([]*signing.Certificate, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := certSelectQuery + " WHERE user_id = $1"
	args := []interface{}{userID}
	if statusFilter != "" {
		query += " AND status = $2"
		args = append(args, statusFilter)
	}
	query += " ORDER BY created_at DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list certificates: %w", err)
	}
	defer rows.Close()

	var result []*signing.Certificate
	for rows.Next() {
		c, err := scanCertificate(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

func (s *PostgresCertificateStore) UpdateStatus(id, status string, revokedAt *time.Time, reason string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := s.db.ExecContext(ctx, `
		UPDATE signing_certificates
		SET status = $1, revoked_at = $2, revocation_reason = $3, updated_at = NOW()
		WHERE id = $4`,
		status, nullTimePtr(revokedAt), nullStrSign(reason), id)
	if err != nil {
		return fmt.Errorf("update certificate status: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("certificate not found: %s", id)
	}
	return nil
}

const certSelectQuery = `
	SELECT id, user_id, serial_number, issuer_dn, subject_dn, status,
		valid_from, valid_to, certificate_pem, fingerprint_sha256,
		revoked_at, COALESCE(revocation_reason,''), created_at, updated_at
	FROM signing_certificates`

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanCertificate(s rowScanner) (*signing.Certificate, error) {
	var c signing.Certificate
	var revokedAt sql.NullTime
	if err := s.Scan(
		&c.ID, &c.UserID, &c.SerialNumber, &c.IssuerDN, &c.SubjectDN, &c.Status,
		&c.ValidFrom, &c.ValidTo, &c.CertificatePEM, &c.FingerprintSHA256,
		&revokedAt, &c.RevocationReason, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if revokedAt.Valid {
		t := revokedAt.Time
		c.RevokedAt = &t
	}
	return &c, nil
}

func nullStrSign(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func nullTimePtr(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return *t
}
