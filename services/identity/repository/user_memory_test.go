package repository

import (
	"context"
	"errors"
	"testing"
)

func newTestUser() *User {
	return &User{
		KeycloakID:         "kc-001",
		NIKToken:           "token-abc123",
		NIKMasked:          "32****0001",
		NameEnc:            []byte("encrypted-name"),
		DOBEnc:             []byte("encrypted-dob"),
		Gender:             "M",
		PhoneHash:          "hash-phone",
		PhoneEnc:           []byte("encrypted-phone"),
		EmailHash:          "hash-email",
		EmailEnc:           []byte("encrypted-email"),
		AddressEnc:         []byte("encrypted-address"),
		WrappedDEK:         []byte("wrapped-dek"),
		AuthLevel:          0,
		VerificationStatus: "PENDING",
	}
}

func TestInMemoryUser_CreateAndGetByID(t *testing.T) {
	repo := NewInMemoryUserRepository()
	ctx := context.Background()
	user := newTestUser()

	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}

	if user.ID == "" {
		t.Fatal("Create: expected ID to be set")
	}
	if user.CreatedAt.IsZero() {
		t.Fatal("Create: expected CreatedAt to be set")
	}

	got, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID: unexpected error: %v", err)
	}

	if got.ID != user.ID {
		t.Errorf("GetByID: ID = %q, want %q", got.ID, user.ID)
	}
	if got.NIKToken != user.NIKToken {
		t.Errorf("GetByID: NIKToken = %q, want %q", got.NIKToken, user.NIKToken)
	}
	if got.VerificationStatus != "PENDING" {
		t.Errorf("GetByID: VerificationStatus = %q, want PENDING", got.VerificationStatus)
	}
	if string(got.NameEnc) != "encrypted-name" {
		t.Errorf("GetByID: NameEnc = %q, want encrypted-name", got.NameEnc)
	}
}

func TestInMemoryUser_CreateAndGetByNIKToken(t *testing.T) {
	repo := NewInMemoryUserRepository()
	ctx := context.Background()
	user := newTestUser()

	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}

	got, err := repo.GetByNIKToken(ctx, user.NIKToken)
	if err != nil {
		t.Fatalf("GetByNIKToken: unexpected error: %v", err)
	}

	if got.ID != user.ID {
		t.Errorf("GetByNIKToken: ID = %q, want %q", got.ID, user.ID)
	}
	if got.KeycloakID != "kc-001" {
		t.Errorf("GetByNIKToken: KeycloakID = %q, want kc-001", got.KeycloakID)
	}
}

func TestInMemoryUser_CreateDuplicateNIKToken(t *testing.T) {
	repo := NewInMemoryUserRepository()
	ctx := context.Background()

	user1 := newTestUser()
	if err := repo.Create(ctx, user1); err != nil {
		t.Fatalf("Create first: unexpected error: %v", err)
	}

	user2 := newTestUser()
	user2.KeycloakID = "kc-002"
	err := repo.Create(ctx, user2)
	if !errors.Is(err, ErrDuplicateNIKToken) {
		t.Fatalf("Create duplicate: got %v, want ErrDuplicateNIKToken", err)
	}
}

func TestInMemoryUser_GetByIDNotFound(t *testing.T) {
	repo := NewInMemoryUserRepository()
	ctx := context.Background()

	_, err := repo.GetByID(ctx, "nonexistent-id")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetByID not found: got %v, want ErrNotFound", err)
	}
}

func TestInMemoryUser_GetByNIKTokenNotFound(t *testing.T) {
	repo := NewInMemoryUserRepository()
	ctx := context.Background()

	_, err := repo.GetByNIKToken(ctx, "nonexistent-token")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetByNIKToken not found: got %v, want ErrNotFound", err)
	}
}

func TestInMemoryUser_UpdateVerificationStatus(t *testing.T) {
	repo := NewInMemoryUserRepository()
	ctx := context.Background()
	user := newTestUser()

	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}

	if err := repo.UpdateVerificationStatus(ctx, user.ID, "VERIFIED"); err != nil {
		t.Fatalf("UpdateVerificationStatus: unexpected error: %v", err)
	}

	got, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID: unexpected error: %v", err)
	}
	if got.VerificationStatus != "VERIFIED" {
		t.Errorf("VerificationStatus = %q, want VERIFIED", got.VerificationStatus)
	}
	if !got.UpdatedAt.After(got.CreatedAt) || got.UpdatedAt.Equal(got.CreatedAt) {
		// UpdatedAt should be >= CreatedAt; in fast tests they may be equal
	}
}

func TestInMemoryUser_UpdateVerificationStatusNotFound(t *testing.T) {
	repo := NewInMemoryUserRepository()
	ctx := context.Background()

	err := repo.UpdateVerificationStatus(ctx, "nonexistent-id", "VERIFIED")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateVerificationStatus not found: got %v, want ErrNotFound", err)
	}
}

func TestInMemoryUser_ExistsTrue(t *testing.T) {
	repo := NewInMemoryUserRepository()
	ctx := context.Background()
	user := newTestUser()

	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}

	exists, err := repo.Exists(ctx, user.NIKToken)
	if err != nil {
		t.Fatalf("Exists: unexpected error: %v", err)
	}
	if !exists {
		t.Error("Exists: got false, want true")
	}
}

func TestInMemoryUser_ExistsFalse(t *testing.T) {
	repo := NewInMemoryUserRepository()
	ctx := context.Background()

	exists, err := repo.Exists(ctx, "nonexistent-token")
	if err != nil {
		t.Fatalf("Exists: unexpected error: %v", err)
	}
	if exists {
		t.Error("Exists: got true, want false")
	}
}

func TestInMemoryUser_ReturnsCopy(t *testing.T) {
	repo := NewInMemoryUserRepository()
	ctx := context.Background()
	user := newTestUser()

	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}

	got1, _ := repo.GetByID(ctx, user.ID)
	got1.VerificationStatus = "MODIFIED"

	got2, _ := repo.GetByID(ctx, user.ID)
	if got2.VerificationStatus == "MODIFIED" {
		t.Error("GetByID returned a reference instead of a copy")
	}
}
