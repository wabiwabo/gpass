package store

import (
	"context"
	"sync"
	"testing"
)

func TestConcurrentEntityCreate(t *testing.T) {
	s := NewInMemoryEntityStore()
	ctx := context.Background()
	var wg sync.WaitGroup
	n := 100

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			e := &Entity{
				AHUSKNumber: "SK-" + string(rune('A'+idx%26)),
				Name:        "PT Test Corp",
				EntityType:  "PT",
				Status:      "ACTIVE",
			}
			if err := s.Create(ctx, e); err != nil {
				t.Errorf("Create %d: %v", idx, err)
			}
		}(i)
	}
	wg.Wait()
}

func TestConcurrentEntityReadWrite(t *testing.T) {
	s := NewInMemoryEntityStore()
	ctx := context.Background()

	// Create base entities
	ids := make([]string, 20)
	for i := 0; i < 20; i++ {
		e := &Entity{Name: "Test Corp", EntityType: "PT", Status: "ACTIVE"}
		_ = s.Create(ctx, e)
		ids[i] = e.ID
	}

	var wg sync.WaitGroup
	// Readers
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _ = s.GetByID(ctx, ids[idx%20])
		}(i)
	}
	// Writers (add officers)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_ = s.AddOfficers(ctx, ids[idx%20], []EntityOfficer{
				{Name: "Officer", Position: "DIREKTUR"},
			})
		}(i)
	}
	wg.Wait()
}

func TestConcurrentAddShareholders(t *testing.T) {
	s := NewInMemoryEntityStore()
	ctx := context.Background()

	e := &Entity{Name: "PT Concurrent", EntityType: "PT", Status: "ACTIVE"}
	_ = s.Create(ctx, e)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_ = s.AddShareholders(ctx, e.ID, []EntityShareholder{
				{Name: "Shareholder", Percentage: 5.0},
			})
		}(i)
	}
	wg.Wait()

	got, _ := s.GetByID(ctx, e.ID)
	if len(got.Shareholders) != 50 {
		t.Errorf("shareholders: got %d, want 50", len(got.Shareholders))
	}
}

func TestEntityCopyIsolation(t *testing.T) {
	s := NewInMemoryEntityStore()
	ctx := context.Background()

	e := &Entity{
		Name: "PT Original", EntityType: "PT", Status: "ACTIVE",
		Officers: []EntityOfficer{{Name: "Alice", Position: "DIREKTUR"}},
	}
	_ = s.Create(ctx, e)

	got, _ := s.GetByID(ctx, e.ID)
	got.Officers[0].Name = "TAMPERED"
	got.Name = "TAMPERED"

	got2, _ := s.GetByID(ctx, e.ID)
	if got2.Name == "TAMPERED" {
		t.Error("name should not be mutated")
	}
	if got2.Officers[0].Name == "TAMPERED" {
		t.Error("officers should not be mutated")
	}
}

func TestEntityGetBySKNumber(t *testing.T) {
	s := NewInMemoryEntityStore()
	ctx := context.Background()

	e := &Entity{AHUSKNumber: "AHU-12345", Name: "PT Test", EntityType: "PT"}
	_ = s.Create(ctx, e)

	got, err := s.GetBySKNumber(ctx, "AHU-12345")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got.AHUSKNumber != "AHU-12345" {
		t.Errorf("SK: got %q", got.AHUSKNumber)
	}
}

func TestEntityGetBySKNotFound(t *testing.T) {
	s := NewInMemoryEntityStore()
	_, err := s.GetBySKNumber(context.Background(), "NONEXISTENT")
	if err != ErrEntityNotFound {
		t.Errorf("got %v, want ErrEntityNotFound", err)
	}
}

func TestAddOfficersNotFound(t *testing.T) {
	s := NewInMemoryEntityStore()
	err := s.AddOfficers(context.Background(), "bad-id", []EntityOfficer{{Name: "X"}})
	if err != ErrEntityNotFound {
		t.Errorf("got %v, want ErrEntityNotFound", err)
	}
}

func TestAddShareholdersNotFound(t *testing.T) {
	s := NewInMemoryEntityStore()
	err := s.AddShareholders(context.Background(), "bad-id", []EntityShareholder{{Name: "X"}})
	if err != ErrEntityNotFound {
		t.Errorf("got %v, want ErrEntityNotFound", err)
	}
}

func TestAddOfficersAssignsIDs(t *testing.T) {
	s := NewInMemoryEntityStore()
	ctx := context.Background()

	e := &Entity{Name: "PT IDs", EntityType: "PT"}
	_ = s.Create(ctx, e)

	officers := []EntityOfficer{
		{Name: "Alice", Position: "DIREKTUR"},
		{Name: "Bob", Position: "KOMISARIS"},
	}
	_ = s.AddOfficers(ctx, e.ID, officers)

	got, _ := s.GetByID(ctx, e.ID)
	for _, o := range got.Officers {
		if o.ID == "" {
			t.Error("officer ID should be assigned")
		}
		if o.EntityID != e.ID {
			t.Errorf("officer EntityID: got %q, want %q", o.EntityID, e.ID)
		}
	}
}

func TestEntityLifecycle(t *testing.T) {
	s := NewInMemoryEntityStore()
	ctx := context.Background()

	// Create
	e := &Entity{
		AHUSKNumber: "AHU-LIFECYCLE",
		Name:        "PT Lifecycle",
		EntityType:  "PT",
		Status:      "ACTIVE",
		NPWP:        "01.234.567.8-901.000",
		CapitalAuth: 1000000000,
		CapitalPaid: 500000000,
	}
	_ = s.Create(ctx, e)
	if e.ID == "" {
		t.Fatal("ID should be set")
	}

	// Add officers
	_ = s.AddOfficers(ctx, e.ID, []EntityOfficer{
		{Name: "Budi", Position: "DIREKTUR_UTAMA", NIKToken: "tok_abc"},
		{Name: "Sari", Position: "KOMISARIS"},
	})

	// Add shareholders
	_ = s.AddShareholders(ctx, e.ID, []EntityShareholder{
		{Name: "Budi", Percentage: 60.0, ShareType: "SAHAM_BIASA", Shares: 600},
		{Name: "Investor A", Percentage: 40.0, ShareType: "SAHAM_BIASA", Shares: 400},
	})

	// Verify full entity
	got, _ := s.GetByID(ctx, e.ID)
	if len(got.Officers) != 2 {
		t.Errorf("officers: %d", len(got.Officers))
	}
	if len(got.Shareholders) != 2 {
		t.Errorf("shareholders: %d", len(got.Shareholders))
	}
	if got.CapitalAuth != 1000000000 {
		t.Errorf("capital auth: %d", got.CapitalAuth)
	}
}
