package store

import (
	"testing"
	"time"
)

func TestKeyStore_CreateAndGetByHash(t *testing.T) {
	s := NewInMemoryKeyStore()

	key, err := s.Create(&APIKey{
		AppID:       "app-1",
		KeyHash:     "abc123hash",
		KeyPrefix:   "gp_test_abcd1234",
		Name:        "Test Key",
		Environment: "sandbox",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if key.ID == "" {
		t.Error("expected ID to be set")
	}
	if key.Status != "ACTIVE" {
		t.Errorf("expected status ACTIVE, got %s", key.Status)
	}

	// Get by hash
	got, err := s.GetByHash("abc123hash")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != key.ID {
		t.Errorf("expected ID %s, got %s", key.ID, got.ID)
	}
}

func TestKeyStore_GetByHash_NotFound(t *testing.T) {
	s := NewInMemoryKeyStore()

	_, err := s.GetByHash("nonexistent")
	if err != ErrKeyNotFound {
		t.Fatalf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestKeyStore_ListByApp(t *testing.T) {
	s := NewInMemoryKeyStore()

	s.Create(&APIKey{AppID: "app-1", KeyHash: "hash1", KeyPrefix: "gp_test_00000001", Name: "Key 1", Environment: "sandbox"})
	s.Create(&APIKey{AppID: "app-1", KeyHash: "hash2", KeyPrefix: "gp_test_00000002", Name: "Key 2", Environment: "sandbox"})
	s.Create(&APIKey{AppID: "app-2", KeyHash: "hash3", KeyPrefix: "gp_test_00000003", Name: "Key 3", Environment: "sandbox"})

	keys, err := s.ListByApp("app-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

func TestKeyStore_Revoke(t *testing.T) {
	s := NewInMemoryKeyStore()

	key, _ := s.Create(&APIKey{
		AppID:       "app-1",
		KeyHash:     "hash1",
		KeyPrefix:   "gp_test_00000001",
		Name:        "Test",
		Environment: "sandbox",
	})

	err := s.Revoke(key.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := s.GetByHash("hash1")
	if got.Status != "REVOKED" {
		t.Errorf("expected REVOKED, got %s", got.Status)
	}
	if got.RevokedAt == nil {
		t.Error("expected RevokedAt to be set")
	}
}

func TestKeyStore_Revoke_NotFound(t *testing.T) {
	s := NewInMemoryKeyStore()

	err := s.Revoke("nonexistent")
	if err != ErrKeyNotFound {
		t.Fatalf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestKeyStore_UpdateLastUsed(t *testing.T) {
	s := NewInMemoryKeyStore()

	key, _ := s.Create(&APIKey{
		AppID:       "app-1",
		KeyHash:     "hash1",
		KeyPrefix:   "gp_test_00000001",
		Name:        "Test",
		Environment: "sandbox",
	})

	err := s.UpdateLastUsed(key.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := s.GetByHash("hash1")
	if got.LastUsedAt == nil {
		t.Error("expected LastUsedAt to be set")
	}
	if time.Since(*got.LastUsedAt) > time.Second {
		t.Error("LastUsedAt should be recent")
	}
}
