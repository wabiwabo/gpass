package session_test

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/garudapass/gpass/apps/bff/session"
	"github.com/redis/go-redis/v9"
)

func setupRedisStore(t *testing.T, encrypt bool) (*session.RedisStore, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	var key []byte
	if encrypt {
		key = make([]byte, 32)
		rand.Read(key)
	}

	store, err := session.NewRedisStore(rdb, key)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	return store, mr
}

func TestRedisStoreCreateAndGet(t *testing.T) {
	store, _ := setupRedisStore(t, false)
	ctx := context.Background()

	data := &session.Data{
		UserID:       "redis-user-1",
		AccessToken:  "access-token-value",
		RefreshToken: "refresh-token-value",
		IDToken:      "id-token-value",
		CSRFToken:    "csrf-token-value",
		ExpiresAt:    time.Now().Add(30 * time.Minute).Truncate(time.Millisecond),
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

	if got.UserID != "redis-user-1" {
		t.Errorf("UserID: got %s, want redis-user-1", got.UserID)
	}
	if got.AccessToken != "access-token-value" {
		t.Errorf("AccessToken mismatch")
	}
	if got.RefreshToken != "refresh-token-value" {
		t.Errorf("RefreshToken mismatch")
	}
	if got.CSRFToken != "csrf-token-value" {
		t.Errorf("CSRFToken mismatch")
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt should be auto-set")
	}
}

func TestRedisStoreWithEncryption(t *testing.T) {
	store, mr := setupRedisStore(t, true)
	ctx := context.Background()

	data := &session.Data{
		UserID:       "encrypted-user",
		AccessToken:  "secret-access-token",
		RefreshToken: "secret-refresh-token",
	}

	sid, err := store.Create(ctx, data, 30*time.Minute)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify data is encrypted in Redis (not plaintext)
	raw, err := mr.Get("gpass:session:" + sid)
	if err != nil {
		t.Fatalf("failed to read raw Redis data: %v", err)
	}

	// Raw data should NOT contain the access token in plaintext
	if containsSubstring(raw, "secret-access-token") {
		t.Error("access token found in plaintext in Redis — encryption is not working!")
	}

	// But we should be able to decrypt it via the store
	got, err := store.Get(ctx, sid)
	if err != nil {
		t.Fatalf("Get with decryption failed: %v", err)
	}
	if got.AccessToken != "secret-access-token" {
		t.Errorf("decrypted AccessToken mismatch: got %s", got.AccessToken)
	}
}

func TestRedisStoreUpdate(t *testing.T) {
	store, _ := setupRedisStore(t, true)
	ctx := context.Background()

	sid, _ := store.Create(ctx, &session.Data{
		UserID:      "user-before",
		AccessToken: "old-token",
	}, 30*time.Minute)

	err := store.Update(ctx, sid, &session.Data{
		UserID:      "user-before",
		AccessToken: "new-token",
	}, 30*time.Minute)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	got, _ := store.Get(ctx, sid)
	if got.AccessToken != "new-token" {
		t.Errorf("expected new-token after update, got %s", got.AccessToken)
	}
}

func TestRedisStoreDelete(t *testing.T) {
	store, _ := setupRedisStore(t, false)
	ctx := context.Background()

	sid, _ := store.Create(ctx, &session.Data{UserID: "to-delete"}, 30*time.Minute)

	err := store.Delete(ctx, sid)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = store.Get(ctx, sid)
	if err != session.ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound after delete, got %v", err)
	}
}

func TestRedisStoreGetNonExistent(t *testing.T) {
	store, _ := setupRedisStore(t, false)
	ctx := context.Background()

	// Valid format but doesn't exist
	_, err := store.Get(ctx, "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2")
	if err != session.ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestRedisStoreTTLExpiry(t *testing.T) {
	store, mr := setupRedisStore(t, false)
	ctx := context.Background()

	sid, _ := store.Create(ctx, &session.Data{UserID: "ttl-user"}, 1*time.Second)

	// Fast-forward time in miniredis
	mr.FastForward(2 * time.Second)

	_, err := store.Get(ctx, sid)
	if err != session.ErrSessionNotFound {
		t.Errorf("expected session to expire after TTL, got %v", err)
	}
}

func TestRedisStoreDeleteInvalidIDSilent(t *testing.T) {
	store, _ := setupRedisStore(t, false)
	ctx := context.Background()

	// Delete with invalid ID should not error
	err := store.Delete(ctx, "invalid-id")
	if err != nil {
		t.Errorf("Delete with invalid ID should be silent, got %v", err)
	}
}

func TestRedisStoreUpdateInvalidID(t *testing.T) {
	store, _ := setupRedisStore(t, false)
	ctx := context.Background()

	err := store.Update(ctx, "bad-id", &session.Data{}, 30*time.Minute)
	if err != session.ErrInvalidID {
		t.Errorf("expected ErrInvalidID, got %v", err)
	}
}

func TestRedisStoreGetInvalidID(t *testing.T) {
	store, _ := setupRedisStore(t, false)
	ctx := context.Background()

	_, err := store.Get(ctx, "not-a-valid-hex-id")
	if err != session.ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound for invalid ID, got %v", err)
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && findSubstring(s, sub)
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
