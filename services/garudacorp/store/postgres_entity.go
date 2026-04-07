package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// PostgresEntityStore is a PostgreSQL-backed implementation of EntityStore.
// 12factor Factor VI compliant.
type PostgresEntityStore struct {
	db *sql.DB
}

func NewPostgresEntityStore(db *sql.DB) *PostgresEntityStore {
	return &PostgresEntityStore{db: db}
}

func (s *PostgresEntityStore) Create(ctx context.Context, e *Entity) error {
	if err := ValidateEntity(e); err != nil {
		return err
	}
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	now := time.Now().UTC()
	e.CreatedAt = now
	e.UpdatedAt = now

	query := `
		INSERT INTO entities (
			ahu_sk_number, name, entity_type, status, npwp, address,
			capital_authorized, capital_paid, ahu_verified_at, oss_nib, oss_verified_at,
			created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		RETURNING id`

	return s.db.QueryRowContext(cctx, query,
		e.AHUSKNumber, e.Name, e.EntityType, e.Status,
		nullStr(e.NPWP), nullStr(e.Address),
		e.CapitalAuth, e.CapitalPaid, e.AHUVerifiedAt,
		nullStr(e.OSSNIB), nullTime(e.OSSVerifiedAt),
		e.CreatedAt, e.UpdatedAt,
	).Scan(&e.ID)
}

func (s *PostgresEntityStore) GetByID(ctx context.Context, id string) (*Entity, error) {
	return s.getEntity(ctx, "id = $1", id)
}

func (s *PostgresEntityStore) GetBySKNumber(ctx context.Context, sk string) (*Entity, error) {
	return s.getEntity(ctx, "ahu_sk_number = $1", sk)
}

func (s *PostgresEntityStore) getEntity(ctx context.Context, where string, arg interface{}) (*Entity, error) {
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := fmt.Sprintf(`
		SELECT id, ahu_sk_number, name, entity_type, status,
			COALESCE(npwp,''), COALESCE(address,''),
			COALESCE(capital_authorized,0), COALESCE(capital_paid,0),
			COALESCE(ahu_verified_at, NOW()), COALESCE(oss_nib,''), oss_verified_at,
			created_at, updated_at
		FROM entities WHERE %s`, where)

	var e Entity
	var ossVerifiedAt sql.NullTime
	err := s.db.QueryRowContext(cctx, query, arg).Scan(
		&e.ID, &e.AHUSKNumber, &e.Name, &e.EntityType, &e.Status,
		&e.NPWP, &e.Address, &e.CapitalAuth, &e.CapitalPaid,
		&e.AHUVerifiedAt, &e.OSSNIB, &ossVerifiedAt,
		&e.CreatedAt, &e.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrEntityNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get entity: %w", err)
	}
	if ossVerifiedAt.Valid {
		t := ossVerifiedAt.Time
		e.OSSVerifiedAt = &t
	}

	// Load officers
	officers, err := s.loadOfficers(cctx, e.ID)
	if err != nil {
		return nil, err
	}
	e.Officers = officers

	// Load shareholders
	shareholders, err := s.loadShareholders(cctx, e.ID)
	if err != nil {
		return nil, err
	}
	e.Shareholders = shareholders

	return &e, nil
}

func (s *PostgresEntityStore) loadOfficers(ctx context.Context, entityID string) ([]EntityOfficer, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, entity_id, COALESCE(user_id::text,''), nik_token, name, position,
			COALESCE(appointment_date::text,''), verified
		FROM entity_officers WHERE entity_id = $1`, entityID)
	if err != nil {
		return nil, fmt.Errorf("load officers: %w", err)
	}
	defer rows.Close()

	var result []EntityOfficer
	for rows.Next() {
		var o EntityOfficer
		if err := rows.Scan(&o.ID, &o.EntityID, &o.UserID, &o.NIKToken,
			&o.Name, &o.Position, &o.AppointmentDate, &o.Verified); err != nil {
			return nil, err
		}
		result = append(result, o)
	}
	return result, rows.Err()
}

func (s *PostgresEntityStore) loadShareholders(ctx context.Context, entityID string) ([]EntityShareholder, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, entity_id, name, COALESCE(share_type,''), COALESCE(shares,0), COALESCE(percentage,0)
		FROM entity_shareholders WHERE entity_id = $1`, entityID)
	if err != nil {
		return nil, fmt.Errorf("load shareholders: %w", err)
	}
	defer rows.Close()

	var result []EntityShareholder
	for rows.Next() {
		var sh EntityShareholder
		if err := rows.Scan(&sh.ID, &sh.EntityID, &sh.Name, &sh.ShareType,
			&sh.Shares, &sh.Percentage); err != nil {
			return nil, err
		}
		result = append(result, sh)
	}
	return result, rows.Err()
}

func (s *PostgresEntityStore) AddOfficers(ctx context.Context, entityID string, officers []EntityOfficer) error {
	for i := range officers {
		if err := ValidateOfficer(&officers[i]); err != nil {
			return err
		}
	}
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	tx, err := s.db.BeginTx(cctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Verify entity exists
	var exists bool
	if err := tx.QueryRowContext(cctx, `SELECT EXISTS(SELECT 1 FROM entities WHERE id = $1)`, entityID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrEntityNotFound
	}

	for i := range officers {
		officers[i].EntityID = entityID
		var appointmentDate interface{}
		if officers[i].AppointmentDate != "" {
			appointmentDate = officers[i].AppointmentDate
		}
		err := tx.QueryRowContext(cctx, `
			INSERT INTO entity_officers (entity_id, nik_token, name, position, appointment_date, verified)
			VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
			entityID, officers[i].NIKToken, officers[i].Name, officers[i].Position,
			appointmentDate, officers[i].Verified,
		).Scan(&officers[i].ID)
		if err != nil {
			return fmt.Errorf("insert officer: %w", err)
		}
	}

	if _, err := tx.ExecContext(cctx, `UPDATE entities SET updated_at = NOW() WHERE id = $1`, entityID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PostgresEntityStore) AddShareholders(ctx context.Context, entityID string, shareholders []EntityShareholder) error {
	for i := range shareholders {
		if err := ValidateShareholder(&shareholders[i]); err != nil {
			return err
		}
	}
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	tx, err := s.db.BeginTx(cctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var exists bool
	if err := tx.QueryRowContext(cctx, `SELECT EXISTS(SELECT 1 FROM entities WHERE id = $1)`, entityID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrEntityNotFound
	}

	for i := range shareholders {
		shareholders[i].EntityID = entityID
		err := tx.QueryRowContext(cctx, `
			INSERT INTO entity_shareholders (entity_id, name, share_type, shares, percentage)
			VALUES ($1,$2,$3,$4,$5) RETURNING id`,
			entityID, shareholders[i].Name, shareholders[i].ShareType,
			shareholders[i].Shares, shareholders[i].Percentage,
		).Scan(&shareholders[i].ID)
		if err != nil {
			return fmt.Errorf("insert shareholder: %w", err)
		}
	}

	if _, err := tx.ExecContext(cctx, `UPDATE entities SET updated_at = NOW() WHERE id = $1`, entityID); err != nil {
		return err
	}
	return tx.Commit()
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func nullTime(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return *t
}
