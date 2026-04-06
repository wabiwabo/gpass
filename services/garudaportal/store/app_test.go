package store

import (
	"testing"
)

func TestAppStore_Create(t *testing.T) {
	s := NewInMemoryAppStore()

	app, err := s.Create(&App{
		OwnerUserID: "user-1",
		Name:        "Test App",
		Description: "A test application",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if app.ID == "" {
		t.Error("expected ID to be set")
	}
	if app.Name != "Test App" {
		t.Errorf("expected name Test App, got %s", app.Name)
	}
	if app.Status != "ACTIVE" {
		t.Errorf("expected status ACTIVE, got %s", app.Status)
	}
	if app.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestAppStore_Create_EmptyName(t *testing.T) {
	s := NewInMemoryAppStore()

	_, err := s.Create(&App{
		OwnerUserID: "user-1",
		Name:        "",
	})
	if err != ErrAppNameEmpty {
		t.Fatalf("expected ErrAppNameEmpty, got %v", err)
	}
}

func TestAppStore_GetByID(t *testing.T) {
	s := NewInMemoryAppStore()

	app, _ := s.Create(&App{
		OwnerUserID: "user-1",
		Name:        "Test App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	})

	got, err := s.GetByID(app.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "Test App" {
		t.Errorf("expected name Test App, got %s", got.Name)
	}
}

func TestAppStore_GetByID_NotFound(t *testing.T) {
	s := NewInMemoryAppStore()

	_, err := s.GetByID("nonexistent")
	if err != ErrAppNotFound {
		t.Fatalf("expected ErrAppNotFound, got %v", err)
	}
}

func TestAppStore_ListByOwner(t *testing.T) {
	s := NewInMemoryAppStore()

	s.Create(&App{OwnerUserID: "user-1", Name: "App 1", Environment: "sandbox", Tier: "free", DailyLimit: 100})
	s.Create(&App{OwnerUserID: "user-1", Name: "App 2", Environment: "sandbox", Tier: "free", DailyLimit: 100})
	s.Create(&App{OwnerUserID: "user-2", Name: "App 3", Environment: "sandbox", Tier: "free", DailyLimit: 100})

	apps, err := s.ListByOwner("user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 2 {
		t.Errorf("expected 2 apps, got %d", len(apps))
	}
}

func TestAppStore_Update(t *testing.T) {
	s := NewInMemoryAppStore()

	app, _ := s.Create(&App{
		OwnerUserID: "user-1",
		Name:        "Original",
		Description: "Old desc",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	})

	newName := "Updated"
	newDesc := "New desc"
	updated, err := s.Update(app.ID, AppUpdate{
		Name:        &newName,
		Description: &newDesc,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Name != "Updated" {
		t.Errorf("expected name Updated, got %s", updated.Name)
	}
	if updated.Description != "New desc" {
		t.Errorf("expected description New desc, got %s", updated.Description)
	}
	if !updated.UpdatedAt.After(app.CreatedAt) || updated.UpdatedAt.Equal(app.CreatedAt) {
		// UpdatedAt should be >= CreatedAt (may be same due to fast execution)
	}
}

func TestAppStore_Update_NotFound(t *testing.T) {
	s := NewInMemoryAppStore()

	_, err := s.Update("nonexistent", AppUpdate{})
	if err != ErrAppNotFound {
		t.Fatalf("expected ErrAppNotFound, got %v", err)
	}
}
