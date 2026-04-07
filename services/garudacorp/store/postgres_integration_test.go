//go:build integration

package store

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/garudapass/gpass/services/garudacorp/ubo"
	_ "github.com/lib/pq"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM beneficial_owners WHERE source = 'INTEGRATION_TEST'")
		_, _ = db.Exec("DELETE FROM entity_shareholders WHERE source = 'INTEGRATION_TEST'")
		_, _ = db.Exec("DELETE FROM entity_officers WHERE source = 'INTEGRATION_TEST'")
		_, _ = db.Exec("DELETE FROM entity_roles WHERE entity_id IN (SELECT id FROM entities WHERE ahu_sk_number LIKE 'INT-TEST-%')")
		_, _ = db.Exec("DELETE FROM entities WHERE ahu_sk_number LIKE 'INT-TEST-%'")
		db.Close()
	})
	return db
}

func TestPostgresEntityStore_CreateGet(t *testing.T) {
	db := openTestDB(t)
	s := NewPostgresEntityStore(db)
	ctx := context.Background()

	e := &Entity{
		AHUSKNumber:   "INT-TEST-001",
		Name:          "PT Integration",
		EntityType:    "PT",
		Status:        "ACTIVE",
		NPWP:          "01.234.567.8-901.000",
		Address:       "Jakarta",
		CapitalAuth:   1000000000,
		CapitalPaid:   500000000,
		AHUVerifiedAt: time.Now().UTC(),
	}
	if err := s.Create(ctx, e); err != nil {
		t.Fatalf("create: %v", err)
	}
	if e.ID == "" {
		t.Fatal("expected ID")
	}

	got, err := s.GetBySKNumber(ctx, "INT-TEST-001")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "PT Integration" || got.CapitalAuth != 1000000000 {
		t.Errorf("mismatch: %+v", got)
	}
}

func TestPostgresRoleStore_AssignRevoke(t *testing.T) {
	db := openTestDB(t)
	es := NewPostgresEntityStore(db)
	rs := NewPostgresRoleStore(db)
	ctx := context.Background()

	e := &Entity{
		AHUSKNumber:   "INT-TEST-002",
		Name:          "PT Roles",
		EntityType:    "PT",
		Status:        "ACTIVE",
		AHUVerifiedAt: time.Now().UTC(),
	}
	if err := es.Create(ctx, e); err != nil {
		t.Fatalf("create entity: %v", err)
	}

	r := &EntityRole{
		EntityID:      e.ID,
		UserID:        "00000000-0000-0000-0000-000000000020",
		Role:          RoleAdmin,
		ServiceAccess: []string{"signing"},
	}
	if err := rs.Assign(ctx, r); err != nil {
		t.Fatalf("assign: %v", err)
	}

	got, err := rs.GetUserRole(ctx, e.ID, r.UserID)
	if err != nil || got.Role != RoleAdmin {
		t.Errorf("get user role: err=%v role=%v", err, got)
	}

	if err := rs.Revoke(ctx, r.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if err := rs.Revoke(ctx, r.ID); err != ErrRoleAlreadyRevoked {
		t.Errorf("double revoke: %v", err)
	}
}

func TestPostgresUBOStore_SaveGet(t *testing.T) {
	db := openTestDB(t)
	es := NewPostgresEntityStore(db)
	us := NewPostgresUBOStore(db)
	ctx := context.Background()

	e := &Entity{
		AHUSKNumber:   "INT-TEST-003",
		Name:          "PT UBO",
		EntityType:    "PT",
		Status:        "ACTIVE",
		AHUVerifiedAt: time.Now().UTC(),
	}
	if err := es.Create(ctx, e); err != nil {
		t.Fatalf("create entity: %v", err)
	}

	result := &ubo.AnalysisResult{
		EntityID:   e.ID,
		EntityName: "PT UBO",
		Criteria:   "PP_13_2018",
		Status:     "IDENTIFIED",
		BeneficialOwners: []ubo.BeneficialOwner{
			{Name: "Budi", NIKToken: "tok1", OwnershipType: "DIRECT_SHARES", Percentage: 60.0, Source: "INTEGRATION_TEST"},
			{Name: "Siti", NIKToken: "tok2", OwnershipType: "DIRECT_SHARES", Percentage: 30.0, Source: "INTEGRATION_TEST"},
		},
	}
	if err := us.Save(result); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := us.GetByEntityID(e.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got.BeneficialOwners) != 2 {
		t.Fatalf("expected 2 UBOs, got %d", len(got.BeneficialOwners))
	}
	if got.BeneficialOwners[0].Percentage != 60.0 {
		t.Errorf("first percentage = %v, want 60.0", got.BeneficialOwners[0].Percentage)
	}
}
