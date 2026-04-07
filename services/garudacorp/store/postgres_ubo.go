package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/garudapass/gpass/services/garudacorp/ubo"
)

// PostgresUBOStore is a PostgreSQL-backed implementation of UBOStore.
// PP 13/2018 beneficial ownership persistence.
type PostgresUBOStore struct {
	db *sql.DB
}

func NewPostgresUBOStore(db *sql.DB) *PostgresUBOStore {
	return &PostgresUBOStore{db: db}
}

// Save replaces all beneficial owners for an entity with the new analysis result.
func (s *PostgresUBOStore) Save(result *ubo.AnalysisResult) error {
	if err := ValidateUBOResult(result); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM beneficial_owners WHERE entity_id = $1`, result.EntityID); err != nil {
		return fmt.Errorf("clear ubo: %w", err)
	}

	for _, bo := range result.BeneficialOwners {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO beneficial_owners (
				entity_id, name, nik_token, ownership_type, percentage,
				source, criteria, status, analyzed_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW())`,
			result.EntityID, bo.Name, bo.NIKToken, bo.OwnershipType,
			bo.Percentage, bo.Source, result.Criteria, result.Status,
		); err != nil {
			return fmt.Errorf("insert ubo: %w", err)
		}
	}
	return tx.Commit()
}

func (s *PostgresUBOStore) GetByEntityID(entityID string) (*ubo.AnalysisResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, `
		SELECT name, nik_token, ownership_type, COALESCE(percentage,0),
			source, criteria, status, analyzed_at
		FROM beneficial_owners WHERE entity_id = $1
		ORDER BY percentage DESC`, entityID)
	if err != nil {
		return nil, fmt.Errorf("query ubo: %w", err)
	}
	defer rows.Close()

	result := &ubo.AnalysisResult{
		EntityID:         entityID,
		BeneficialOwners: []ubo.BeneficialOwner{},
	}
	found := false
	for rows.Next() {
		var bo ubo.BeneficialOwner
		var analyzedAt time.Time
		if err := rows.Scan(&bo.Name, &bo.NIKToken, &bo.OwnershipType,
			&bo.Percentage, &bo.Source, &result.Criteria, &result.Status, &analyzedAt); err != nil {
			return nil, err
		}
		bo.VerifiedAt = analyzedAt.Format(time.RFC3339)
		result.AnalyzedAt = bo.VerifiedAt
		result.BeneficialOwners = append(result.BeneficialOwners, bo)
		found = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrUBONotFound
	}
	return result, nil
}

func (s *PostgresUBOStore) ListAll() ([]*ubo.AnalysisResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT entity_id FROM beneficial_owners`)
	if err != nil {
		return nil, fmt.Errorf("list ubo entities: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	results := make([]*ubo.AnalysisResult, 0, len(ids))
	for _, id := range ids {
		r, err := s.GetByEntityID(id)
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}
