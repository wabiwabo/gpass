package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
)

// TxFunc is a function that executes within a transaction.
type TxFunc func(tx *sql.Tx) error

// WithTx executes fn within a transaction. Commits on success, rolls back on error or panic.
func (p *Pool) WithTx(ctx context.Context, fn TxFunc) (err error) {
	tx, err := p.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("database: begin transaction: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				slog.Error("database: rollback after panic failed", "error", rbErr)
			}
			err = fmt.Errorf("database: panic in transaction: %v", r)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			slog.Error("database: rollback failed", "error", rbErr)
		}
		return fmt.Errorf("database: transaction failed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("database: commit failed: %w", err)
	}

	return nil
}
