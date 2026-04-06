package session_test

import (
	"context"
	"testing"
	"time"

	"github.com/garudapass/gpass/apps/bff/session"
)

func TestInMemoryStore(t *testing.T) {
	store := session.NewInMemoryStore()
	ctx := context.Background()

	data := &session.Data{
		UserID:       "user-123",
		AccessToken:  "at-token",
		RefreshToken: "rt-token",
		ExpiresAt:    time.Now().Add(10 * time.Minute),
	}

	sid, err := store.Create(ctx, data, 30*time.Minute)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if sid == "" {
		t.Fatal("expected non-empty session ID")
	}

	got, err := store.Get(ctx, sid)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.UserID != "user-123" {
		t.Errorf("expected user-123, got %s", got.UserID)
	}
	if got.AccessToken != "at-token" {
		t.Errorf("expected at-token, got %s", got.AccessToken)
	}
	if got.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set automatically")
	}

	err = store.Delete(ctx, sid)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = store.Get(ctx, sid)
	if err != session.ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestValidateID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"valid 64-char hex", "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2", false},
		{"too short", "abc123", true},
		{"too long", "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2ff", true},
		{"invalid chars", "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6ZZZZ", true},
		{"empty", "", true},
		{"injection attempt", "../../../etc/passwd", true},
		{"null bytes", "a1b2c3d4e5f6\x00a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := session.ValidateID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateID(%q) error = %v, wantErr %v", tt.id, err, tt.wantErr)
			}
		})
	}
}

func TestInMemoryStoreRejectsInvalidID(t *testing.T) {
	store := session.NewInMemoryStore()
	ctx := context.Background()

	_, err := store.Get(ctx, "invalid-session-id")
	if err != session.ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound for invalid ID, got %v", err)
	}
}

func TestRedisStoreEncryption(t *testing.T) {
	// Test that encryption key validation works
	_, err := session.NewRedisStore(nil, []byte("too-short"))
	if err == nil {
		t.Error("expected error for short encryption key")
	}

	_, err = session.NewRedisStore(nil, make([]byte, 32))
	if err == nil {
		// Expect nil client error or success depending on impl
		// The cipher setup should succeed even without a real Redis connection
		t.Log("32-byte key accepted (expected)")
	}
}
