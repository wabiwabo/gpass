package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// PostgresRoleStore is a PostgreSQL-backed implementation of RoleStore.
type PostgresRoleStore struct {
	db *sql.DB
}

func NewPostgresRoleStore(db *sql.DB) *PostgresRoleStore {
	return &PostgresRoleStore{db: db}
}

func (s *PostgresRoleStore) Assign(ctx context.Context, r *EntityRole) error {
	if err := ValidateRole(r); err != nil {
		return err
	}
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if r.ServiceAccess == nil {
		r.ServiceAccess = []string{}
	}
	saJSON, err := json.Marshal(r.ServiceAccess)
	if err != nil {
		return fmt.Errorf("marshal service_access: %w", err)
	}

	r.Status = StatusActive
	r.GrantedAt = time.Now().UTC()

	var grantedBy interface{}
	if r.GrantedBy != "" {
		grantedBy = r.GrantedBy
	}

	return s.db.QueryRowContext(cctx, `
		INSERT INTO entity_roles (entity_id, user_id, role, granted_by, service_access, status, granted_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		r.EntityID, r.UserID, r.Role, grantedBy, saJSON, r.Status, r.GrantedAt,
	).Scan(&r.ID)
}

func (s *PostgresRoleStore) ListByEntity(ctx context.Context, entityID string) ([]*EntityRole, error) {
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	rows, err := s.db.QueryContext(cctx, `
		SELECT id, entity_id, user_id, role, COALESCE(granted_by::text,''),
			service_access, status, granted_at, revoked_at
		FROM entity_roles WHERE entity_id = $1`, entityID)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	defer rows.Close()

	var result []*EntityRole
	for rows.Next() {
		r, err := scanRole(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *PostgresRoleStore) GetByID(ctx context.Context, id string) (*EntityRole, error) {
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	row := s.db.QueryRowContext(cctx, `
		SELECT id, entity_id, user_id, role, COALESCE(granted_by::text,''),
			service_access, status, granted_at, revoked_at
		FROM entity_roles WHERE id = $1`, id)

	r, err := scanRole(row)
	if err == sql.ErrNoRows {
		return nil, ErrRoleNotFound
	}
	return r, err
}

func (s *PostgresRoleStore) Revoke(ctx context.Context, id string) error {
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	now := time.Now().UTC()
	res, err := s.db.ExecContext(cctx, `
		UPDATE entity_roles SET status = $1, revoked_at = $2
		WHERE id = $3 AND status != $1`, StatusRevoked, now, id)
	if err != nil {
		return fmt.Errorf("revoke role: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		var status string
		err := s.db.QueryRowContext(cctx, `SELECT status FROM entity_roles WHERE id = $1`, id).Scan(&status)
		if err == sql.ErrNoRows {
			return ErrRoleNotFound
		}
		if err != nil {
			return err
		}
		if status == StatusRevoked {
			return ErrRoleAlreadyRevoked
		}
	}
	return nil
}

func (s *PostgresRoleStore) GetUserRole(ctx context.Context, entityID, userID string) (*EntityRole, error) {
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	row := s.db.QueryRowContext(cctx, `
		SELECT id, entity_id, user_id, role, COALESCE(granted_by::text,''),
			service_access, status, granted_at, revoked_at
		FROM entity_roles
		WHERE entity_id = $1 AND user_id = $2 AND status = 'ACTIVE'
		LIMIT 1`, entityID, userID)

	r, err := scanRole(row)
	if err == sql.ErrNoRows {
		return nil, ErrRoleNotFound
	}
	return r, err
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanRole(s rowScanner) (*EntityRole, error) {
	var r EntityRole
	var saJSON []byte
	var revokedAt sql.NullTime
	if err := s.Scan(&r.ID, &r.EntityID, &r.UserID, &r.Role, &r.GrantedBy,
		&saJSON, &r.Status, &r.GrantedAt, &revokedAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(saJSON, &r.ServiceAccess); err != nil {
		return nil, fmt.Errorf("unmarshal service_access: %w", err)
	}
	if revokedAt.Valid {
		t := revokedAt.Time
		r.RevokedAt = &t
	}
	return &r, nil
}
