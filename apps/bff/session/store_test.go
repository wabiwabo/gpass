package session_test

import (
	"context"
	"crypto/rand"
	"encoding/json"
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

func TestInMemoryStoreUpdate(t *testing.T) {
	store := session.NewInMemoryStore()
	ctx := context.Background()

	sid, _ := store.Create(ctx, &session.Data{
		UserID: "user-original",
	}, 30*time.Minute)

	err := store.Update(ctx, sid, &session.Data{
		UserID:      "user-updated",
		AccessToken: "new-token",
	}, 30*time.Minute)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	got, _ := store.Get(ctx, sid)
	if got.UserID != "user-updated" {
		t.Errorf("expected user-updated, got %s", got.UserID)
	}
	if got.AccessToken != "new-token" {
		t.Errorf("expected new-token, got %s", got.AccessToken)
	}
}

func TestInMemoryStoreIsolation(t *testing.T) {
	store := session.NewInMemoryStore()
	ctx := context.Background()

	original := &session.Data{UserID: "user-1", AccessToken: "token-1"}
	sid, _ := store.Create(ctx, original, 30*time.Minute)

	// Mutating the original should not affect the stored copy
	original.AccessToken = "mutated"

	got, _ := store.Get(ctx, sid)
	if got.AccessToken != "token-1" {
		t.Error("store should return a copy, not a reference to the original")
	}

	// Mutating the returned value should not affect the stored copy
	got.AccessToken = "also-mutated"
	got2, _ := store.Get(ctx, sid)
	if got2.AccessToken != "token-1" {
		t.Error("store should return independent copies on each Get")
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
		{"sql injection", "' OR 1=1; DROP TABLE sessions; --                         ", true},
		{"uppercase hex", "A1B2C3D4E5F6A1B2C3D4E5F6A1B2C3D4E5F6A1B2C3D4E5F6A1B2C3D4E5F6A1B2", true},
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

	err = store.Update(ctx, "invalid-session-id", &session.Data{}, 30*time.Minute)
	if err != session.ErrInvalidID {
		t.Errorf("expected ErrInvalidID for Update with invalid ID, got %v", err)
	}
}

func TestRedisStoreEncryptionKeyValidation(t *testing.T) {
	_, err := session.NewRedisStore(nil, []byte("too-short"))
	if err == nil {
		t.Error("expected error for short encryption key")
	}

	_, err = session.NewRedisStore(nil, make([]byte, 16))
	if err == nil {
		t.Error("expected error for 16-byte key (need 32)")
	}

	store, err := session.NewRedisStore(nil, make([]byte, 32))
	if err != nil {
		t.Fatalf("expected 32-byte key to be accepted, got: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}

	// nil key should also work (no encryption)
	store, err = session.NewRedisStore(nil, nil)
	if err != nil {
		t.Fatalf("expected nil key to be accepted, got: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store with nil key")
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)

	store, err := session.NewRedisStore(nil, key)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	testData := &session.Data{
		UserID:       "user-encrypt-test",
		AccessToken:  "access-token-secret-value",
		RefreshToken: "refresh-token-secret-value",
		IDToken:      "id-token-value",
		CSRFToken:    "csrf-token-value",
		ExpiresAt:    time.Now().Add(30 * time.Minute).Truncate(time.Millisecond),
	}

	// Marshal → Encrypt → Decrypt → Unmarshal
	plaintext, err := json.Marshal(testData)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	ciphertext, err := store.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	// Ciphertext should be different from plaintext
	if string(ciphertext) == string(plaintext) {
		t.Error("ciphertext should differ from plaintext")
	}

	// Ciphertext should be longer (nonce + auth tag overhead)
	if len(ciphertext) <= len(plaintext) {
		t.Errorf("ciphertext (%d bytes) should be longer than plaintext (%d bytes)", len(ciphertext), len(plaintext))
	}

	decrypted, err := store.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}

	var result session.Data
	if err := json.Unmarshal(decrypted, &result); err != nil {
		t.Fatalf("unmarshal decrypted data failed: %v", err)
	}

	if result.UserID != testData.UserID {
		t.Errorf("UserID mismatch: got %s, want %s", result.UserID, testData.UserID)
	}
	if result.AccessToken != testData.AccessToken {
		t.Errorf("AccessToken mismatch after round-trip")
	}
	if result.RefreshToken != testData.RefreshToken {
		t.Errorf("RefreshToken mismatch after round-trip")
	}
}

func TestEncryptProducesUniqueOutputs(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)

	store, _ := session.NewRedisStore(nil, key)

	plaintext := []byte(`{"user_id":"same-data"}`)

	ct1, _ := store.Encrypt(plaintext)
	ct2, _ := store.Encrypt(plaintext)

	// Same plaintext should produce different ciphertexts (random nonce)
	if string(ct1) == string(ct2) {
		t.Error("encrypting the same plaintext twice should produce different ciphertexts (IND-CPA)")
	}
}

func TestDecryptRejectsTamperedData(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)

	store, _ := session.NewRedisStore(nil, key)

	plaintext := []byte(`{"user_id":"test"}`)
	ciphertext, _ := store.Encrypt(plaintext)

	// Tamper with the ciphertext
	tampered := make([]byte, len(ciphertext))
	copy(tampered, ciphertext)
	tampered[len(tampered)-1] ^= 0xFF // Flip bits in the auth tag

	_, err := store.Decrypt(tampered)
	if err == nil {
		t.Error("expected error when decrypting tampered ciphertext")
	}
}

func TestDecryptRejectsTruncatedData(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)

	store, _ := session.NewRedisStore(nil, key)

	_, err := store.Decrypt([]byte("short"))
	if err == nil {
		t.Error("expected error for truncated ciphertext")
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	rand.Read(key1)
	rand.Read(key2)

	store1, _ := session.NewRedisStore(nil, key1)
	store2, _ := session.NewRedisStore(nil, key2)

	plaintext := []byte(`{"user_id":"test"}`)
	ciphertext, _ := store1.Encrypt(plaintext)

	_, err := store2.Decrypt(ciphertext)
	if err == nil {
		t.Error("expected error when decrypting with wrong key")
	}
}

func TestNoEncryptionPassthrough(t *testing.T) {
	store, _ := session.NewRedisStore(nil, nil)

	plaintext := []byte(`{"user_id":"test"}`)
	result, err := store.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("passthrough encrypt failed: %v", err)
	}
	if string(result) != string(plaintext) {
		t.Error("with no key, Encrypt should return plaintext unchanged")
	}

	decrypted, err := store.Decrypt(result)
	if err != nil {
		t.Fatalf("passthrough decrypt failed: %v", err)
	}
	if string(decrypted) != string(plaintext) {
		t.Error("with no key, Decrypt should return data unchanged")
	}
}
