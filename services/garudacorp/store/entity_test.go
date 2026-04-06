package store

import (
	"context"
	"testing"
)

func TestEntityStore_CreateAndGetByID(t *testing.T) {
	s := NewInMemoryEntityStore()
	ctx := context.Background()

	e := &Entity{
		AHUSKNumber: "AHU-12345",
		Name:        "PT Test Corp",
		EntityType:  "PT",
		Status:      "ACTIVE",
		NPWP:        "01.234.567.8-901.000",
		Address:     "Jakarta",
		CapitalAuth: 1000000000,
		CapitalPaid: 500000000,
	}

	if err := s.Create(ctx, e); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if e.ID == "" {
		t.Fatal("expected ID to be set")
	}

	got, err := s.GetByID(ctx, e.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "PT Test Corp" {
		t.Errorf("Name = %q, want %q", got.Name, "PT Test Corp")
	}
	if got.AHUSKNumber != "AHU-12345" {
		t.Errorf("AHUSKNumber = %q, want %q", got.AHUSKNumber, "AHU-12345")
	}
	if got.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestEntityStore_GetBySKNumber(t *testing.T) {
	s := NewInMemoryEntityStore()
	ctx := context.Background()

	e := &Entity{
		AHUSKNumber: "AHU-67890",
		Name:        "PT Another Corp",
		EntityType:  "PT",
		Status:      "ACTIVE",
	}
	s.Create(ctx, e)

	got, err := s.GetBySKNumber(ctx, "AHU-67890")
	if err != nil {
		t.Fatalf("GetBySKNumber: %v", err)
	}
	if got.Name != "PT Another Corp" {
		t.Errorf("Name = %q, want %q", got.Name, "PT Another Corp")
	}
}

func TestEntityStore_GetByID_NotFound(t *testing.T) {
	s := NewInMemoryEntityStore()
	ctx := context.Background()

	_, err := s.GetByID(ctx, "nonexistent")
	if err != ErrEntityNotFound {
		t.Errorf("expected ErrEntityNotFound, got %v", err)
	}
}

func TestEntityStore_GetBySKNumber_NotFound(t *testing.T) {
	s := NewInMemoryEntityStore()
	ctx := context.Background()

	_, err := s.GetBySKNumber(ctx, "AHU-NOPE")
	if err != ErrEntityNotFound {
		t.Errorf("expected ErrEntityNotFound, got %v", err)
	}
}

func TestEntityStore_AddOfficers(t *testing.T) {
	s := NewInMemoryEntityStore()
	ctx := context.Background()

	e := &Entity{
		AHUSKNumber: "AHU-12345",
		Name:        "PT Test Corp",
		EntityType:  "PT",
		Status:      "ACTIVE",
	}
	s.Create(ctx, e)

	officers := []EntityOfficer{
		{NIKToken: "token1", Name: "John Doe", Position: "DIREKTUR_UTAMA", AppointmentDate: "2020-01-01"},
		{NIKToken: "token2", Name: "Jane Doe", Position: "KOMISARIS", AppointmentDate: "2020-01-01"},
	}
	if err := s.AddOfficers(ctx, e.ID, officers); err != nil {
		t.Fatalf("AddOfficers: %v", err)
	}

	got, err := s.GetByID(ctx, e.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if len(got.Officers) != 2 {
		t.Fatalf("expected 2 officers, got %d", len(got.Officers))
	}
	if got.Officers[0].ID == "" {
		t.Error("expected officer ID to be set")
	}
	if got.Officers[0].EntityID != e.ID {
		t.Errorf("EntityID = %q, want %q", got.Officers[0].EntityID, e.ID)
	}
}

func TestEntityStore_AddOfficers_NotFound(t *testing.T) {
	s := NewInMemoryEntityStore()
	ctx := context.Background()

	err := s.AddOfficers(ctx, "nonexistent", []EntityOfficer{{Name: "Test"}})
	if err != ErrEntityNotFound {
		t.Errorf("expected ErrEntityNotFound, got %v", err)
	}
}

func TestEntityStore_AddShareholders(t *testing.T) {
	s := NewInMemoryEntityStore()
	ctx := context.Background()

	e := &Entity{
		AHUSKNumber: "AHU-12345",
		Name:        "PT Test Corp",
		EntityType:  "PT",
		Status:      "ACTIVE",
	}
	s.Create(ctx, e)

	shareholders := []EntityShareholder{
		{Name: "John Doe", ShareType: "INDIVIDUAL", Shares: 500, Percentage: 50.0},
		{Name: "PT Holding", ShareType: "CORPORATE", Shares: 500, Percentage: 50.0},
	}
	if err := s.AddShareholders(ctx, e.ID, shareholders); err != nil {
		t.Fatalf("AddShareholders: %v", err)
	}

	got, err := s.GetByID(ctx, e.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if len(got.Shareholders) != 2 {
		t.Fatalf("expected 2 shareholders, got %d", len(got.Shareholders))
	}
}
