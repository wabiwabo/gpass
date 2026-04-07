package store

import (
	"context"
	"database/sql"
	"testing"

	"github.com/garudapass/gpass/services/garudacorp/ubo"
)

// TestPostgresEntityStore_GetBySKNumber_NotFound pins the SK-lookup path.
func TestPostgresEntityStore_GetBySKNumber_NotFound(t *testing.T) {
	db, _ := sql.Open("gc-fake-ok", "")
	defer db.Close()
	s := NewPostgresEntityStore(db)
	if _, err := s.GetBySKNumber(context.Background(), "AHU-001"); err == nil {
		t.Error("expected not-found")
	}
}

// TestPostgresEntityStore_AddOfficers_ValidationFail pins the per-officer
// validation short-circuit before any DB call.
func TestPostgresEntityStore_AddOfficers_ValidationFail(t *testing.T) {
	db, _ := sql.Open("gc-fake-ok", "")
	defer db.Close()
	s := NewPostgresEntityStore(db)
	err := s.AddOfficers(context.Background(), "e1", []EntityOfficer{{}})
	if err == nil {
		t.Error("expected validation error")
	}
}

// TestPostgresEntityStore_AddOfficers_BeginTxError pins the begin-tx wrap.
func TestPostgresEntityStore_AddOfficers_BeginTxError(t *testing.T) {
	db, _ := sql.Open("gc-fake-bad", "")
	defer db.Close()
	s := NewPostgresEntityStore(db)
	err := s.AddOfficers(context.Background(), "e1", []EntityOfficer{
		{NIKToken: "t1", Name: "Budi", Position: "DIRECTOR"},
	})
	if err == nil {
		t.Error("expected begin error")
	}
}

// TestPostgresEntityStore_AddShareholders_ValidationFail pins the guard.
func TestPostgresEntityStore_AddShareholders_ValidationFail(t *testing.T) {
	db, _ := sql.Open("gc-fake-ok", "")
	defer db.Close()
	s := NewPostgresEntityStore(db)
	err := s.AddShareholders(context.Background(), "e1", []EntityShareholder{{}})
	if err == nil {
		t.Error("expected validation error")
	}
}

// TestPostgresEntityStore_AddShareholders_BeginTxError pins the begin wrap.
func TestPostgresEntityStore_AddShareholders_BeginTxError(t *testing.T) {
	db, _ := sql.Open("gc-fake-bad", "")
	defer db.Close()
	s := NewPostgresEntityStore(db)
	err := s.AddShareholders(context.Background(), "e1", []EntityShareholder{
		{Name: "Budi", ShareType: "INDIVIDUAL", Shares: 100, Percentage: 100},
	})
	if err == nil {
		t.Error("expected begin error")
	}
}

// TestPostgresRoleStore_ListByEntity_DriverError pins the list wrap.
func TestPostgresRoleStore_ListByEntity_DriverError(t *testing.T) {
	db, _ := sql.Open("gc-fake-bad", "")
	defer db.Close()
	s := NewPostgresRoleStore(db)
	if _, err := s.ListByEntity(context.Background(), "e1"); err == nil {
		t.Error("expected error")
	}
}

// TestPostgresRoleStore_ListByEntity_Empty pins empty-rows iteration.
func TestPostgresRoleStore_ListByEntity_Empty(t *testing.T) {
	db, _ := sql.Open("gc-fake-ok", "")
	defer db.Close()
	s := NewPostgresRoleStore(db)
	roles, err := s.ListByEntity(context.Background(), "e1")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(roles) != 0 {
		t.Errorf("roles = %d", len(roles))
	}
}

// TestPostgresRoleStore_GetByID_NotFound pins ErrRoleNotFound.
func TestPostgresRoleStore_GetByID_NotFound(t *testing.T) {
	db, _ := sql.Open("gc-fake-ok", "")
	defer db.Close()
	s := NewPostgresRoleStore(db)
	if _, err := s.GetByID(context.Background(), "id"); err != ErrRoleNotFound {
		t.Errorf("err = %v", err)
	}
}

// TestPostgresRoleStore_Revoke_DriverError pins the UPDATE wrap.
func TestPostgresRoleStore_Revoke_DriverError(t *testing.T) {
	db, _ := sql.Open("gc-fake-bad", "")
	defer db.Close()
	s := NewPostgresRoleStore(db)
	if err := s.Revoke(context.Background(), "id"); err == nil {
		t.Error("expected error")
	}
}

// TestPostgresRoleStore_Revoke_NotFound pins the 0-rows + ErrNoRows path.
func TestPostgresRoleStore_Revoke_NotFound(t *testing.T) {
	db, _ := sql.Open("gc-fake-ok", "")
	defer db.Close()
	s := NewPostgresRoleStore(db)
	if err := s.Revoke(context.Background(), "missing"); err != ErrRoleNotFound {
		t.Errorf("err = %v", err)
	}
}

// TestPostgresRoleStore_GetUserRole_NotFound pins the "no active role" branch.
func TestPostgresRoleStore_GetUserRole_NotFound(t *testing.T) {
	db, _ := sql.Open("gc-fake-ok", "")
	defer db.Close()
	s := NewPostgresRoleStore(db)
	if _, err := s.GetUserRole(context.Background(), "e1", "u1"); err != ErrRoleNotFound {
		t.Errorf("err = %v", err)
	}
}

// TestPostgresUBOStore_Save_ValidationFail pins the pre-DB guard.
func TestPostgresUBOStore_Save_ValidationFail(t *testing.T) {
	db, _ := sql.Open("gc-fake-ok", "")
	defer db.Close()
	s := NewPostgresUBOStore(db)
	if err := s.Save(&ubo.AnalysisResult{}); err == nil {
		t.Error("expected validation error")
	}
}

// TestPostgresUBOStore_Save_BeginTxError pins the begin-tx wrap.
func TestPostgresUBOStore_Save_BeginTxError(t *testing.T) {
	db, _ := sql.Open("gc-fake-bad", "")
	defer db.Close()
	s := NewPostgresUBOStore(db)
	err := s.Save(&ubo.AnalysisResult{
		EntityID: "e1",
		Criteria: "ownership_25",
		Status:   "COMPLETED",
		BeneficialOwners: []ubo.BeneficialOwner{
			{Name: "Budi", NIKToken: "t1", OwnershipType: "DIRECT", Percentage: 30, Source: "AHU"},
		},
	})
	if err == nil {
		t.Error("expected error")
	}
}

// TestPostgresUBOStore_GetByEntityID_NotFound pins ErrUBONotFound.
func TestPostgresUBOStore_GetByEntityID_NotFound(t *testing.T) {
	db, _ := sql.Open("gc-fake-ok", "")
	defer db.Close()
	s := NewPostgresUBOStore(db)
	if _, err := s.GetByEntityID("e1"); err != ErrUBONotFound {
		t.Errorf("err = %v", err)
	}
}

// TestPostgresUBOStore_GetByEntityID_DriverError pins the query wrap.
func TestPostgresUBOStore_GetByEntityID_DriverError(t *testing.T) {
	db, _ := sql.Open("gc-fake-bad", "")
	defer db.Close()
	s := NewPostgresUBOStore(db)
	if _, err := s.GetByEntityID("e1"); err == nil {
		t.Error("expected error")
	}
}

// TestPostgresUBOStore_ListAll_DriverError pins the distinct-query wrap.
func TestPostgresUBOStore_ListAll_DriverError(t *testing.T) {
	db, _ := sql.Open("gc-fake-bad", "")
	defer db.Close()
	s := NewPostgresUBOStore(db)
	if _, err := s.ListAll(); err == nil {
		t.Error("expected error")
	}
}

// TestPostgresUBOStore_ListAll_Empty pins the happy-empty path.
func TestPostgresUBOStore_ListAll_Empty(t *testing.T) {
	db, _ := sql.Open("gc-fake-ok", "")
	defer db.Close()
	s := NewPostgresUBOStore(db)
	list, err := s.ListAll()
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(list) != 0 {
		t.Errorf("list = %d", len(list))
	}
}

// TestNewPostgresUBOStore pins the constructor that was 0%.
func TestNewPostgresUBOStore(t *testing.T) {
	db, _ := sql.Open("gc-fake-ok", "")
	defer db.Close()
	if NewPostgresUBOStore(db) == nil {
		t.Error("nil store")
	}
}
