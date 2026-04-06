package repository

import (
	"context"
	"errors"
	"testing"
	"time"
)

func newTestAPIKey() *APIKeyRecord {
	expires := time.Now().Add(30 * 24 * time.Hour)
	return &APIKeyRecord{
		AppID:       "app-001",
		KeyHash:     "sha256-hash-abc123",
		KeyPrefix:   "gp_live_abc1",
		Name:        "Production Key",
		Environment: "production",
		Status:      "active",
		ExpiresAt:   &expires,
	}
}

func TestInMemoryAPIKey_CreateAndGetByHash(t *testing.T) {
	repo := NewInMemoryAPIKeyRepository()
	ctx := context.Background()
	key := newTestAPIKey()

	if err := repo.Create(ctx, key); err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}

	if key.ID == "" {
		t.Fatal("Create: expected ID to be set")
	}
	if key.CreatedAt.IsZero() {
		t.Fatal("Create: expected CreatedAt to be set")
	}

	got, err := repo.GetByHash(ctx, key.KeyHash)
	if err != nil {
		t.Fatalf("GetByHash: unexpected error: %v", err)
	}

	if got.ID != key.ID {
		t.Errorf("GetByHash: ID = %q, want %q", got.ID, key.ID)
	}
	if got.KeyPrefix != "gp_live_abc1" {
		t.Errorf("GetByHash: KeyPrefix = %q, want gp_live_abc1", got.KeyPrefix)
	}
	if got.Name != "Production Key" {
		t.Errorf("GetByHash: Name = %q, want Production Key", got.Name)
	}
	if got.Status != "active" {
		t.Errorf("GetByHash: Status = %q, want active", got.Status)
	}
}

func TestInMemoryAPIKey_GetByHashNotFound(t *testing.T) {
	repo := NewInMemoryAPIKeyRepository()
	ctx := context.Background()

	_, err := repo.GetByHash(ctx, "nonexistent-hash")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetByHash not found: got %v, want ErrNotFound", err)
	}
}

func TestInMemoryAPIKey_ListByApp(t *testing.T) {
	repo := NewInMemoryAPIKeyRepository()
	ctx := context.Background()

	key1 := newTestAPIKey()
	key1.KeyHash = "hash-1"
	key1.Name = "Key One"
	if err := repo.Create(ctx, key1); err != nil {
		t.Fatalf("Create key1: %v", err)
	}

	key2 := newTestAPIKey()
	key2.KeyHash = "hash-2"
	key2.Name = "Key Two"
	if err := repo.Create(ctx, key2); err != nil {
		t.Fatalf("Create key2: %v", err)
	}

	keys, err := repo.ListByApp(ctx, "app-001")
	if err != nil {
		t.Fatalf("ListByApp: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("ListByApp: got %d keys, want 2", len(keys))
	}
}

func TestInMemoryAPIKey_ListByAppEmpty(t *testing.T) {
	repo := NewInMemoryAPIKeyRepository()
	ctx := context.Background()

	keys, err := repo.ListByApp(ctx, "no-such-app")
	if err != nil {
		t.Fatalf("ListByApp: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("ListByApp: got %d keys, want 0", len(keys))
	}
}

func TestInMemoryAPIKey_Revoke(t *testing.T) {
	repo := NewInMemoryAPIKeyRepository()
	ctx := context.Background()
	key := newTestAPIKey()

	if err := repo.Create(ctx, key); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Revoke(ctx, key.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	got, err := repo.GetByHash(ctx, key.KeyHash)
	if err != nil {
		t.Fatalf("GetByHash after revoke: %v", err)
	}
	if got.Status != "revoked" {
		t.Errorf("Status = %q, want revoked", got.Status)
	}
	if got.RevokedAt == nil {
		t.Error("RevokedAt should be set after revoke")
	}
}

func TestInMemoryAPIKey_RevokeNotFound(t *testing.T) {
	repo := NewInMemoryAPIKeyRepository()
	ctx := context.Background()

	err := repo.Revoke(ctx, "nonexistent-id")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Revoke not found: got %v, want ErrNotFound", err)
	}
}

func TestInMemoryAPIKey_RevokeAlreadyRevoked(t *testing.T) {
	repo := NewInMemoryAPIKeyRepository()
	ctx := context.Background()
	key := newTestAPIKey()

	if err := repo.Create(ctx, key); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Revoke(ctx, key.ID); err != nil {
		t.Fatalf("Revoke first: %v", err)
	}

	err := repo.Revoke(ctx, key.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Revoke already revoked: got %v, want ErrNotFound", err)
	}
}

func TestInMemoryAPIKey_UpdateLastUsed(t *testing.T) {
	repo := NewInMemoryAPIKeyRepository()
	ctx := context.Background()
	key := newTestAPIKey()

	if err := repo.Create(ctx, key); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.UpdateLastUsed(ctx, key.ID); err != nil {
		t.Fatalf("UpdateLastUsed: %v", err)
	}

	got, err := repo.GetByHash(ctx, key.KeyHash)
	if err != nil {
		t.Fatalf("GetByHash: %v", err)
	}
	if got.LastUsedAt == nil {
		t.Error("LastUsedAt should be set after UpdateLastUsed")
	}
}

func TestInMemoryAPIKey_UpdateLastUsedNotFound(t *testing.T) {
	repo := NewInMemoryAPIKeyRepository()
	ctx := context.Background()

	err := repo.UpdateLastUsed(ctx, "nonexistent-id")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateLastUsed not found: got %v, want ErrNotFound", err)
	}
}

func TestInMemoryAPIKey_DuplicateHash(t *testing.T) {
	repo := NewInMemoryAPIKeyRepository()
	ctx := context.Background()

	key1 := newTestAPIKey()
	if err := repo.Create(ctx, key1); err != nil {
		t.Fatalf("Create first: %v", err)
	}

	key2 := newTestAPIKey() // same hash
	err := repo.Create(ctx, key2)
	if err == nil {
		t.Fatal("Create duplicate hash: expected error, got nil")
	}
}

func TestInMemoryAPIKey_ReturnsCopy(t *testing.T) {
	repo := NewInMemoryAPIKeyRepository()
	ctx := context.Background()
	key := newTestAPIKey()

	if err := repo.Create(ctx, key); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got1, _ := repo.GetByHash(ctx, key.KeyHash)
	got1.Name = "MODIFIED"

	got2, _ := repo.GetByHash(ctx, key.KeyHash)
	if got2.Name == "MODIFIED" {
		t.Error("GetByHash returned a reference instead of a copy")
	}
}
