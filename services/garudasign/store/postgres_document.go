package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/garudapass/gpass/services/garudasign/signing"
)

// PostgresDocumentStore is a PostgreSQL-backed DocumentStore.
type PostgresDocumentStore struct {
	db *sql.DB
}

func NewPostgresDocumentStore(db *sql.DB) *PostgresDocumentStore {
	return &PostgresDocumentStore{db: db}
}

func (s *PostgresDocumentStore) Create(doc *signing.SignedDocument) (*signing.SignedDocument, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	doc.CreatedAt = time.Now().UTC()

	err := s.db.QueryRowContext(ctx, `
		INSERT INTO signed_documents (
			request_id, certificate_id, signed_hash, signed_path, signed_size,
			pades_level, signature_timestamp, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id`,
		doc.RequestID, doc.CertificateID, doc.SignedHash, doc.SignedPath, doc.SignedSize,
		doc.PAdESLevel, doc.SignatureTimestamp, doc.CreatedAt,
	).Scan(&doc.ID)
	if err != nil {
		return nil, fmt.Errorf("create signed document: %w", err)
	}
	return doc, nil
}

func (s *PostgresDocumentStore) GetByRequestID(requestID string) (*signing.SignedDocument, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var d signing.SignedDocument
	err := s.db.QueryRowContext(ctx, `
		SELECT id, request_id, certificate_id, signed_hash, signed_path, signed_size,
			pades_level, signature_timestamp, created_at
		FROM signed_documents WHERE request_id = $1`, requestID).Scan(
		&d.ID, &d.RequestID, &d.CertificateID, &d.SignedHash, &d.SignedPath, &d.SignedSize,
		&d.PAdESLevel, &d.SignatureTimestamp, &d.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("signed document not found for request: %s", requestID)
	}
	if err != nil {
		return nil, fmt.Errorf("get signed document: %w", err)
	}
	return &d, nil
}
