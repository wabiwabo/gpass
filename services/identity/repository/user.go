package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("record not found")

// ErrDuplicateNIKToken is returned when a NIK token already exists.
var ErrDuplicateNIKToken = errors.New("nik token already exists")

// User represents a user record matching the users table schema.
type User struct {
	ID                 string
	KeycloakID         string
	NIKToken           string
	NIKMasked          string
	NameEnc            []byte
	DOBEnc             []byte
	Gender             string
	PhoneHash          string
	PhoneEnc           []byte
	EmailHash          string
	EmailEnc           []byte
	AddressEnc         []byte
	WrappedDEK         []byte
	AuthLevel          int
	VerificationStatus string
	DukcapilVerifiedAt *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// UserRepository provides user persistence operations.
type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id string) (*User, error)
	GetByNIKToken(ctx context.Context, nikToken string) (*User, error)
	UpdateVerificationStatus(ctx context.Context, id, status string) error
	Exists(ctx context.Context, nikToken string) (bool, error)
}

// PostgresUserRepository implements UserRepository with PostgreSQL.
type PostgresUserRepository struct {
	db *sql.DB
}

// NewPostgresUserRepository creates a new PostgreSQL-backed user repository.
func NewPostgresUserRepository(db *sql.DB) *PostgresUserRepository {
	return &PostgresUserRepository{db: db}
}

// Create inserts a new user and populates id, created_at, and updated_at from the DB.
func (r *PostgresUserRepository) Create(ctx context.Context, user *User) error {
	query := `
		INSERT INTO users (
			keycloak_id, nik_token, nik_masked, name_enc, dob_enc, gender,
			phone_hash, phone_enc, email_hash, email_enc, address_enc,
			wrapped_dek, auth_level, verification_status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id, created_at, updated_at`

	err := r.db.QueryRowContext(ctx, query,
		user.KeycloakID,
		user.NIKToken,
		user.NIKMasked,
		user.NameEnc,
		user.DOBEnc,
		user.Gender,
		user.PhoneHash,
		user.PhoneEnc,
		user.EmailHash,
		user.EmailEnc,
		user.AddressEnc,
		user.WrappedDEK,
		user.AuthLevel,
		user.VerificationStatus,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateNIKToken
		}
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// GetByID retrieves a user by their UUID.
func (r *PostgresUserRepository) GetByID(ctx context.Context, id string) (*User, error) {
	return r.getUser(ctx, "SELECT "+userColumns()+" FROM users WHERE id = $1", id)
}

// GetByNIKToken retrieves a user by their NIK token.
func (r *PostgresUserRepository) GetByNIKToken(ctx context.Context, nikToken string) (*User, error) {
	return r.getUser(ctx, "SELECT "+userColumns()+" FROM users WHERE nik_token = $1", nikToken)
}

// UpdateVerificationStatus updates the verification status and updated_at timestamp.
func (r *PostgresUserRepository) UpdateVerificationStatus(ctx context.Context, id, status string) error {
	query := `UPDATE users SET verification_status = $1, updated_at = NOW() WHERE id = $2`
	result, err := r.db.ExecContext(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("update verification status: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update verification status: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// Exists checks whether a user with the given NIK token exists.
func (r *PostgresUserRepository) Exists(ctx context.Context, nikToken string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE nik_token = $1)`
	if err := r.db.QueryRowContext(ctx, query, nikToken).Scan(&exists); err != nil {
		return false, fmt.Errorf("check user exists: %w", err)
	}
	return exists, nil
}

func (r *PostgresUserRepository) getUser(ctx context.Context, query, arg string) (*User, error) {
	var u User
	err := r.db.QueryRowContext(ctx, query, arg).Scan(
		&u.ID,
		&u.KeycloakID,
		&u.NIKToken,
		&u.NIKMasked,
		&u.NameEnc,
		&u.DOBEnc,
		&u.Gender,
		&u.PhoneHash,
		&u.PhoneEnc,
		&u.EmailHash,
		&u.EmailEnc,
		&u.AddressEnc,
		&u.WrappedDEK,
		&u.AuthLevel,
		&u.VerificationStatus,
		&u.DukcapilVerifiedAt,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &u, nil
}

func userColumns() string {
	return `id, keycloak_id, nik_token, nik_masked, name_enc, dob_enc, gender,
		phone_hash, phone_enc, email_hash, email_enc, address_enc,
		wrapped_dek, auth_level, verification_status, dukcapil_verified_at,
		created_at, updated_at`
}

// isUniqueViolation checks if a PostgreSQL error is a unique constraint violation.
func isUniqueViolation(err error) bool {
	// PostgreSQL unique_violation error code is 23505.
	// We check the error string to avoid importing a PostgreSQL driver package.
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
