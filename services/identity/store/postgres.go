package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// PostgresDeletionStore is a PostgreSQL-backed DeletionStore.
// 12factor Factor VI compliant + UU PDP No. 27/2022 right-to-deletion auditability.
type PostgresDeletionStore struct {
	db *sql.DB
}

func NewPostgresDeletionStore(db *sql.DB) *PostgresDeletionStore {
	return &PostgresDeletionStore{db: db}
}

func (s *PostgresDeletionStore) Create(req *DeletionRequest) error {
	if !ValidReasons[req.Reason] {
		return ErrInvalidReason
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if req.DeletedData == nil {
		req.DeletedData = []string{}
	}
	dataJSON, err := json.Marshal(req.DeletedData)
	if err != nil {
		return fmt.Errorf("marshal deleted_data: %w", err)
	}

	req.Status = "PENDING"
	req.RequestedAt = time.Now().UTC()

	return s.db.QueryRowContext(ctx, `
		INSERT INTO deletion_requests (user_id, reason, status, requested_at, deleted_data)
		VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		req.UserID, req.Reason, req.Status, req.RequestedAt, dataJSON,
	).Scan(&req.ID)
}

func (s *PostgresDeletionStore) GetByID(id string) (*DeletionRequest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := s.db.QueryRowContext(ctx, deletionSelectQuery+" WHERE id = $1", id)
	r, err := scanDeletionRequest(row)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return r, err
}

func (s *PostgresDeletionStore) ListByUser(userID string) ([]*DeletionRequest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := s.db.QueryContext(ctx,
		deletionSelectQuery+" WHERE user_id = $1 ORDER BY requested_at DESC", userID)
	if err != nil {
		return nil, fmt.Errorf("list deletion requests: %w", err)
	}
	defer rows.Close()

	var result []*DeletionRequest
	for rows.Next() {
		r, err := scanDeletionRequest(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *PostgresDeletionStore) UpdateStatus(id, status string, completedAt *time.Time, deletedData []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var (
		ct  interface{}
		dd  interface{}
		err error
	)
	if completedAt != nil {
		ct = *completedAt
	}
	if deletedData != nil {
		dd, err = json.Marshal(deletedData)
		if err != nil {
			return fmt.Errorf("marshal deleted_data: %w", err)
		}
	}

	res, err := s.db.ExecContext(ctx, `
		UPDATE deletion_requests
		SET status = $1,
			completed_at = COALESCE($2, completed_at),
			deleted_data = COALESCE($3::jsonb, deleted_data)
		WHERE id = $4`, status, ct, dd, id)
	if err != nil {
		return fmt.Errorf("update deletion status: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

const deletionSelectQuery = `
	SELECT id, user_id, reason, status, requested_at, completed_at, deleted_data
	FROM deletion_requests`

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanDeletionRequest(s rowScanner) (*DeletionRequest, error) {
	var r DeletionRequest
	var completedAt sql.NullTime
	var dataJSON []byte
	if err := s.Scan(&r.ID, &r.UserID, &r.Reason, &r.Status,
		&r.RequestedAt, &completedAt, &dataJSON); err != nil {
		return nil, err
	}
	if completedAt.Valid {
		t := completedAt.Time
		r.CompletedAt = &t
	}
	if len(dataJSON) > 0 {
		if err := json.Unmarshal(dataJSON, &r.DeletedData); err != nil {
			return nil, fmt.Errorf("unmarshal deleted_data: %w", err)
		}
	}
	return &r, nil
}
