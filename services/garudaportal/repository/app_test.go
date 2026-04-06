package repository

import (
	"context"
	"errors"
	"testing"
)

func newTestApp() *App {
	return &App{
		OwnerUserID:   "owner-001",
		Name:          "My Test App",
		Description:   "A test application",
		Environment:   "sandbox",
		Tier:          "free",
		DailyLimit:    1000,
		CallbackURLs:  []string{"https://example.com/callback"},
		OAuthClientID: "oauth-client-001",
		Status:        "active",
	}
}

func TestInMemoryApp_CreateAndGetByID(t *testing.T) {
	repo := NewInMemoryAppRepository()
	ctx := context.Background()
	app := newTestApp()

	if err := repo.Create(ctx, app); err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}

	if app.ID == "" {
		t.Fatal("Create: expected ID to be set")
	}
	if app.CreatedAt.IsZero() {
		t.Fatal("Create: expected CreatedAt to be set")
	}

	got, err := repo.GetByID(ctx, app.ID)
	if err != nil {
		t.Fatalf("GetByID: unexpected error: %v", err)
	}

	if got.ID != app.ID {
		t.Errorf("GetByID: ID = %q, want %q", got.ID, app.ID)
	}
	if got.Name != "My Test App" {
		t.Errorf("GetByID: Name = %q, want %q", got.Name, "My Test App")
	}
	if got.Status != "active" {
		t.Errorf("GetByID: Status = %q, want active", got.Status)
	}
	if len(got.CallbackURLs) != 1 || got.CallbackURLs[0] != "https://example.com/callback" {
		t.Errorf("GetByID: CallbackURLs = %v, want [https://example.com/callback]", got.CallbackURLs)
	}
}

func TestInMemoryApp_GetByIDNotFound(t *testing.T) {
	repo := NewInMemoryAppRepository()
	ctx := context.Background()

	_, err := repo.GetByID(ctx, "nonexistent-id")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetByID not found: got %v, want ErrNotFound", err)
	}
}

func TestInMemoryApp_ListByOwner(t *testing.T) {
	repo := NewInMemoryAppRepository()
	ctx := context.Background()

	app1 := newTestApp()
	app1.Name = "App One"
	if err := repo.Create(ctx, app1); err != nil {
		t.Fatalf("Create app1: %v", err)
	}

	app2 := newTestApp()
	app2.Name = "App Two"
	if err := repo.Create(ctx, app2); err != nil {
		t.Fatalf("Create app2: %v", err)
	}

	apps, err := repo.ListByOwner(ctx, "owner-001")
	if err != nil {
		t.Fatalf("ListByOwner: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("ListByOwner: got %d apps, want 2", len(apps))
	}
}

func TestInMemoryApp_ListByOwnerEmpty(t *testing.T) {
	repo := NewInMemoryAppRepository()
	ctx := context.Background()

	apps, err := repo.ListByOwner(ctx, "no-such-owner")
	if err != nil {
		t.Fatalf("ListByOwner: %v", err)
	}
	if len(apps) != 0 {
		t.Fatalf("ListByOwner: got %d apps, want 0", len(apps))
	}
}

func TestInMemoryApp_Update(t *testing.T) {
	repo := NewInMemoryAppRepository()
	ctx := context.Background()
	app := newTestApp()

	if err := repo.Create(ctx, app); err != nil {
		t.Fatalf("Create: %v", err)
	}

	updates := map[string]interface{}{
		"name":        "Updated Name",
		"description": "Updated description",
		"daily_limit": 5000,
	}
	if err := repo.Update(ctx, app.ID, updates); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := repo.GetByID(ctx, app.ID)
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}
	if got.Name != "Updated Name" {
		t.Errorf("Name = %q, want %q", got.Name, "Updated Name")
	}
	if got.Description != "Updated description" {
		t.Errorf("Description = %q, want %q", got.Description, "Updated description")
	}
	if got.DailyLimit != 5000 {
		t.Errorf("DailyLimit = %d, want 5000", got.DailyLimit)
	}
}

func TestInMemoryApp_UpdateNotFound(t *testing.T) {
	repo := NewInMemoryAppRepository()
	ctx := context.Background()

	err := repo.Update(ctx, "nonexistent-id", map[string]interface{}{"name": "x"})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Update not found: got %v, want ErrNotFound", err)
	}
}

func TestInMemoryApp_UpdateStatus(t *testing.T) {
	repo := NewInMemoryAppRepository()
	ctx := context.Background()
	app := newTestApp()

	if err := repo.Create(ctx, app); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.UpdateStatus(ctx, app.ID, "suspended"); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	got, err := repo.GetByID(ctx, app.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != "suspended" {
		t.Errorf("Status = %q, want suspended", got.Status)
	}
}

func TestInMemoryApp_UpdateStatusNotFound(t *testing.T) {
	repo := NewInMemoryAppRepository()
	ctx := context.Background()

	err := repo.UpdateStatus(ctx, "nonexistent-id", "suspended")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateStatus not found: got %v, want ErrNotFound", err)
	}
}

func TestInMemoryApp_CreateDuplicate(t *testing.T) {
	repo := NewInMemoryAppRepository()
	ctx := context.Background()

	app1 := newTestApp()
	if err := repo.Create(ctx, app1); err != nil {
		t.Fatalf("Create first: %v", err)
	}

	app2 := newTestApp() // same owner + name
	err := repo.Create(ctx, app2)
	if !errors.Is(err, ErrDuplicateApp) {
		t.Fatalf("Create duplicate: got %v, want ErrDuplicateApp", err)
	}
}

func TestInMemoryApp_ReturnsCopy(t *testing.T) {
	repo := NewInMemoryAppRepository()
	ctx := context.Background()
	app := newTestApp()

	if err := repo.Create(ctx, app); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got1, _ := repo.GetByID(ctx, app.ID)
	got1.Name = "MODIFIED"

	got2, _ := repo.GetByID(ctx, app.ID)
	if got2.Name == "MODIFIED" {
		t.Error("GetByID returned a reference instead of a copy")
	}
}
