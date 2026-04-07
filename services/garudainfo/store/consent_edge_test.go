package store

import (
	"context"
	"testing"
	"time"
)

func TestListActiveByUserAndClient(t *testing.T) {
	s := NewInMemoryConsentStore()
	ctx := context.Background()

	// Create active consent
	c1 := &Consent{
		UserID: "user-1", ClientID: "client-1",
		Fields: map[string]bool{"name": true}, DurationSeconds: 3600,
	}
	_ = s.Create(ctx, c1)

	// Create and revoke another
	c2 := &Consent{
		UserID: "user-1", ClientID: "client-1",
		Fields: map[string]bool{"email": true}, DurationSeconds: 3600,
	}
	_ = s.Create(ctx, c2)
	_ = s.Revoke(ctx, c2.ID)

	// Create for different client
	c3 := &Consent{
		UserID: "user-1", ClientID: "client-2",
		Fields: map[string]bool{"phone": true}, DurationSeconds: 3600,
	}
	_ = s.Create(ctx, c3)

	list, err := s.ListActiveByUserAndClient(ctx, "user-1", "client-1")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("got %d, want 1 (only active for client-1)", len(list))
	}
	if len(list) == 1 && list[0].ID != c1.ID {
		t.Errorf("wrong consent returned: got %s, want %s", list[0].ID, c1.ID)
	}
}

func TestListActiveByUserAndClientEmpty(t *testing.T) {
	s := NewInMemoryConsentStore()
	ctx := context.Background()

	list, err := s.ListActiveByUserAndClient(ctx, "nonexistent", "client")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("got %d, want 0", len(list))
	}
}

func TestListByUserEmpty(t *testing.T) {
	s := NewInMemoryConsentStore()
	ctx := context.Background()

	list, err := s.ListByUser(ctx, "nobody")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("got %d, want 0", len(list))
	}
}

func TestRevokeNotFound(t *testing.T) {
	s := NewInMemoryConsentStore()
	ctx := context.Background()

	err := s.Revoke(ctx, "nonexistent-id")
	if err != ErrConsentNotFound {
		t.Errorf("got %v, want ErrConsentNotFound", err)
	}
}

func TestExpireStaleSkipsRevoked(t *testing.T) {
	s := NewInMemoryConsentStore()
	ctx := context.Background()

	c := &Consent{
		UserID: "user-1", ClientID: "client-1",
		Fields: map[string]bool{"name": true}, DurationSeconds: 1,
	}
	_ = s.Create(ctx, c)
	_ = s.Revoke(ctx, c.ID)

	// Force expiry time
	s.mu.Lock()
	s.consents[c.ID].ExpiresAt = time.Now().UTC().Add(-1 * time.Hour)
	s.mu.Unlock()

	count, _ := s.ExpireStale(ctx)
	if count != 0 {
		t.Errorf("revoked consent should not be expired, got count=%d", count)
	}
}

func TestExpireStaleNoActiveConsents(t *testing.T) {
	s := NewInMemoryConsentStore()
	ctx := context.Background()

	count, err := s.ExpireStale(ctx)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if count != 0 {
		t.Errorf("got %d, want 0", count)
	}
}

func TestCreateAutoSetsTimestamps(t *testing.T) {
	s := NewInMemoryConsentStore()
	ctx := context.Background()

	before := time.Now().UTC().Add(-1 * time.Second)
	c := &Consent{
		UserID: "user-1", ClientID: "client-1",
		Fields: map[string]bool{"name": true}, DurationSeconds: 86400,
	}
	_ = s.Create(ctx, c)
	after := time.Now().UTC().Add(1 * time.Second)

	if c.GrantedAt.Before(before) || c.GrantedAt.After(after) {
		t.Errorf("GrantedAt %v not in expected range", c.GrantedAt)
	}
	expectedExpiry := c.GrantedAt.Add(86400 * time.Second)
	diff := c.ExpiresAt.Sub(expectedExpiry)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("ExpiresAt %v not within 1s of expected %v", c.ExpiresAt, expectedExpiry)
	}
}

func TestCreateUniqueIDs(t *testing.T) {
	s := NewInMemoryConsentStore()
	ctx := context.Background()

	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		c := &Consent{
			UserID: "user-1", ClientID: "client-1",
			Fields: map[string]bool{"name": true}, DurationSeconds: 3600,
		}
		_ = s.Create(ctx, c)
		if ids[c.ID] {
			t.Fatalf("duplicate ID: %s", c.ID)
		}
		ids[c.ID] = true
	}
}

func TestCopyConsentDeepCopiesFields(t *testing.T) {
	original := &Consent{
		Fields: map[string]bool{"name": true, "email": false},
	}

	cp := copyConsent(original)
	cp.Fields["name"] = false
	cp.Fields["phone"] = true

	if !original.Fields["name"] {
		t.Error("mutating copy should not affect original name field")
	}
	if _, exists := original.Fields["phone"]; exists {
		t.Error("adding to copy should not affect original")
	}
}

func TestCopyConsentDeepCopiesRevokedAt(t *testing.T) {
	now := time.Now().UTC()
	original := &Consent{
		RevokedAt: &now,
	}

	cp := copyConsent(original)
	newTime := now.Add(1 * time.Hour)
	cp.RevokedAt = &newTime

	if !original.RevokedAt.Equal(now) {
		t.Error("mutating copy RevokedAt should not affect original")
	}
}

func TestCopyConsentNilRevokedAt(t *testing.T) {
	original := &Consent{
		Fields: map[string]bool{"name": true},
	}
	cp := copyConsent(original)
	if cp.RevokedAt != nil {
		t.Error("copy should preserve nil RevokedAt")
	}
}

func TestListByUserReturnsCopies(t *testing.T) {
	s := NewInMemoryConsentStore()
	ctx := context.Background()

	c := &Consent{
		UserID: "user-copy", ClientID: "client-1",
		Fields: map[string]bool{"name": true}, DurationSeconds: 3600,
	}
	_ = s.Create(ctx, c)

	list, _ := s.ListByUser(ctx, "user-copy")
	list[0].Fields["name"] = false

	// Verify store is not mutated
	got, _ := s.GetByID(ctx, c.ID)
	if !got.Fields["name"] {
		t.Error("mutating list result should not affect store")
	}
}

func TestListActiveByUserAndClientReturnsCopies(t *testing.T) {
	s := NewInMemoryConsentStore()
	ctx := context.Background()

	c := &Consent{
		UserID: "user-copy2", ClientID: "client-1",
		Fields: map[string]bool{"email": true}, DurationSeconds: 3600,
	}
	_ = s.Create(ctx, c)

	list, _ := s.ListActiveByUserAndClient(ctx, "user-copy2", "client-1")
	list[0].Status = "TAMPERED"

	got, _ := s.GetByID(ctx, c.ID)
	if got.Status != "ACTIVE" {
		t.Error("mutating list result should not affect store")
	}
}

func TestMultipleFieldsConsent(t *testing.T) {
	s := NewInMemoryConsentStore()
	ctx := context.Background()

	fields := map[string]bool{
		"name": true, "email": true, "phone": true,
		"nik": true, "dob": true, "address": false,
		"religion": false, "blood_type": false,
	}
	c := &Consent{
		UserID: "user-multi", ClientID: "client-1",
		Fields: fields, DurationSeconds: 3600,
	}
	_ = s.Create(ctx, c)

	got, _ := s.GetByID(ctx, c.ID)
	if len(got.Fields) != 8 {
		t.Errorf("fields count: got %d, want 8", len(got.Fields))
	}
	if !got.Fields["name"] {
		t.Error("name should be true")
	}
	if got.Fields["address"] {
		t.Error("address should be false")
	}
}

func TestConsentLifecycle(t *testing.T) {
	s := NewInMemoryConsentStore()
	ctx := context.Background()

	// 1. Create
	c := &Consent{
		UserID: "user-lifecycle", ClientID: "client-1",
		ClientName: "Test App", Purpose: "KYC",
		Fields: map[string]bool{"name": true, "nik": true},
		DurationSeconds: 3600,
	}
	if err := s.Create(ctx, c); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.Status != "ACTIVE" {
		t.Errorf("status after create: %s", c.Status)
	}

	// 2. Verify active
	active, _ := s.ListActiveByUserAndClient(ctx, "user-lifecycle", "client-1")
	if len(active) != 1 {
		t.Errorf("active count: %d", len(active))
	}

	// 3. Revoke
	if err := s.Revoke(ctx, c.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	// 4. Verify revoked
	got, _ := s.GetByID(ctx, c.ID)
	if got.Status != "REVOKED" {
		t.Errorf("status after revoke: %s", got.Status)
	}
	if got.RevokedAt == nil {
		t.Error("RevokedAt should be set")
	}

	// 5. No longer active
	active, _ = s.ListActiveByUserAndClient(ctx, "user-lifecycle", "client-1")
	if len(active) != 0 {
		t.Errorf("active after revoke: %d", len(active))
	}

	// 6. Still in full list
	all, _ := s.ListByUser(ctx, "user-lifecycle")
	if len(all) != 1 {
		t.Errorf("all consents: %d", len(all))
	}
}
