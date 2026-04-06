package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
)

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("record not found")

// ErrDuplicateApp is returned when an app with the same name already exists for the owner.
var ErrDuplicateApp = errors.New("app already exists")

// App represents a developer application record.
type App struct {
	ID            string
	OwnerUserID   string
	Name          string
	Description   string
	Environment   string
	Tier          string
	DailyLimit    int
	CallbackURLs  []string
	OAuthClientID string
	Status        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// AppRepository provides app persistence operations.
type AppRepository interface {
	Create(ctx context.Context, app *App) error
	GetByID(ctx context.Context, id string) (*App, error)
	ListByOwner(ctx context.Context, ownerUserID string) ([]*App, error)
	Update(ctx context.Context, id string, updates map[string]interface{}) error
	UpdateStatus(ctx context.Context, id, status string) error
}

// PostgresAppRepository implements AppRepository with PostgreSQL.
type PostgresAppRepository struct {
	db *sql.DB
}

// NewPostgresAppRepository creates a new PostgreSQL-backed app repository.
func NewPostgresAppRepository(db *sql.DB) *PostgresAppRepository {
	return &PostgresAppRepository{db: db}
}

// Create inserts a new app and populates id, created_at, and updated_at from the DB.
func (r *PostgresAppRepository) Create(ctx context.Context, app *App) error {
	query := `
		INSERT INTO apps (
			owner_user_id, name, description, environment, tier,
			daily_limit, callback_urls, oauth_client_id, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at`

	err := r.db.QueryRowContext(ctx, query,
		app.OwnerUserID,
		app.Name,
		app.Description,
		app.Environment,
		app.Tier,
		app.DailyLimit,
		pq.Array(app.CallbackURLs),
		app.OAuthClientID,
		app.Status,
	).Scan(&app.ID, &app.CreatedAt, &app.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateApp
		}
		return fmt.Errorf("create app: %w", err)
	}
	return nil
}

// GetByID retrieves an app by its UUID.
func (r *PostgresAppRepository) GetByID(ctx context.Context, id string) (*App, error) {
	query := "SELECT " + appColumns() + " FROM apps WHERE id = $1"
	var a App
	var callbackURLs pq.StringArray
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&a.ID, &a.OwnerUserID, &a.Name, &a.Description,
		&a.Environment, &a.Tier, &a.DailyLimit,
		&callbackURLs, &a.OAuthClientID,
		&a.Status, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get app: %w", err)
	}
	a.CallbackURLs = callbackURLs
	return &a, nil
}

// ListByOwner retrieves all apps for a given owner.
func (r *PostgresAppRepository) ListByOwner(ctx context.Context, ownerUserID string) ([]*App, error) {
	query := "SELECT " + appColumns() + " FROM apps WHERE owner_user_id = $1 ORDER BY created_at DESC"
	rows, err := r.db.QueryContext(ctx, query, ownerUserID)
	if err != nil {
		return nil, fmt.Errorf("list apps: %w", err)
	}
	defer rows.Close()

	var apps []*App
	for rows.Next() {
		var a App
		var callbackURLs pq.StringArray
		if err := rows.Scan(
			&a.ID, &a.OwnerUserID, &a.Name, &a.Description,
			&a.Environment, &a.Tier, &a.DailyLimit,
			&callbackURLs, &a.OAuthClientID,
			&a.Status, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan app: %w", err)
		}
		a.CallbackURLs = callbackURLs
		apps = append(apps, &a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate apps: %w", err)
	}
	return apps, nil
}

// Update applies a map of field updates to an app.
// Supported keys: name, description, environment, tier, daily_limit, callback_urls, status.
func (r *PostgresAppRepository) Update(ctx context.Context, id string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	setClauses := ""
	args := make([]interface{}, 0, len(updates)+1)
	paramIdx := 1

	for key, val := range updates {
		if setClauses != "" {
			setClauses += ", "
		}
		switch key {
		case "callback_urls":
			if urls, ok := val.([]string); ok {
				setClauses += fmt.Sprintf("%s = $%d", key, paramIdx)
				args = append(args, pq.Array(urls))
			} else {
				return fmt.Errorf("invalid type for callback_urls")
			}
		default:
			setClauses += fmt.Sprintf("%s = $%d", key, paramIdx)
			args = append(args, val)
		}
		paramIdx++
	}

	setClauses += fmt.Sprintf(", updated_at = NOW()")
	args = append(args, id)

	query := fmt.Sprintf("UPDATE apps SET %s WHERE id = $%d", setClauses, paramIdx)
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update app: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update app: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateStatus updates the status and updated_at timestamp.
func (r *PostgresAppRepository) UpdateStatus(ctx context.Context, id, status string) error {
	query := `UPDATE apps SET status = $1, updated_at = NOW() WHERE id = $2`
	result, err := r.db.ExecContext(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("update app status: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update app status: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func appColumns() string {
	return `id, owner_user_id, name, description, environment, tier,
		daily_limit, callback_urls, oauth_client_id, status, created_at, updated_at`
}

// isUniqueViolation checks if a PostgreSQL error is a unique constraint violation.
func isUniqueViolation(err error) bool {
	return err != nil && contains(err.Error(), "23505")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
