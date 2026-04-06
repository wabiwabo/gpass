package statuspage

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestManager_SetComponent(t *testing.T) {
	m := NewManager()
	m.SetComponent(Component{Name: "API", Status: StatusOperational})
	m.SetComponent(Component{Name: "DB", Status: StatusOperational})

	if m.ComponentCount() != 2 {
		t.Errorf("count: got %d", m.ComponentCount())
	}
}

func TestManager_AllOperational(t *testing.T) {
	m := NewManager()
	m.SetComponent(Component{Name: "API", Status: StatusOperational})
	m.SetComponent(Component{Name: "DB", Status: StatusOperational})

	page := m.Page()
	if page.Status != StatusOperational {
		t.Errorf("status: got %q", page.Status)
	}
}

func TestManager_DegradedComponent(t *testing.T) {
	m := NewManager()
	m.SetComponent(Component{Name: "API", Status: StatusOperational})
	m.SetComponent(Component{Name: "Cache", Status: StatusDegraded})

	page := m.Page()
	if page.Status != StatusDegraded {
		t.Errorf("status: got %q", page.Status)
	}
}

func TestManager_MajorOutage(t *testing.T) {
	m := NewManager()
	m.SetComponent(Component{Name: "API", Status: StatusOperational})
	m.SetComponent(Component{Name: "DB", Status: StatusMajorOutage})

	page := m.Page()
	if page.Status != StatusMajorOutage {
		t.Errorf("status: got %q", page.Status)
	}
}

func TestManager_UpdateComponentStatus(t *testing.T) {
	m := NewManager()
	m.SetComponent(Component{Name: "API", Status: StatusOperational})
	m.UpdateComponentStatus("API", StatusMajorOutage)

	page := m.Page()
	if page.Components[0].Status != StatusMajorOutage {
		t.Errorf("updated status: got %q", page.Components[0].Status)
	}
}

func TestManager_AddIncident(t *testing.T) {
	m := NewManager()
	m.AddIncident(Incident{
		ID:     "inc-1",
		Title:  "Database slowdown",
		Status: IncidentInvestigating,
		Impact: StatusDegraded,
	})

	page := m.Page()
	if len(page.Incidents) != 1 {
		t.Errorf("incidents: got %d", len(page.Incidents))
	}
	if page.Incidents[0].Title != "Database slowdown" {
		t.Errorf("title: got %q", page.Incidents[0].Title)
	}
}

func TestManager_UpdateIncident(t *testing.T) {
	m := NewManager()
	m.AddIncident(Incident{ID: "inc-1", Title: "Issue", Status: IncidentInvestigating})
	m.UpdateIncident("inc-1", IncidentUpdate{
		Status:  IncidentIdentified,
		Message: "Root cause found",
	})

	page := m.Page()
	if page.Incidents[0].Status != IncidentIdentified {
		t.Errorf("status: got %q", page.Incidents[0].Status)
	}
	if len(page.Incidents[0].Updates) != 1 {
		t.Errorf("updates: got %d", len(page.Incidents[0].Updates))
	}
}

func TestManager_ResolveIncident(t *testing.T) {
	m := NewManager()
	m.AddIncident(Incident{ID: "inc-1", Title: "Issue", Status: IncidentInvestigating})
	m.UpdateIncident("inc-1", IncidentUpdate{Status: IncidentResolved, Message: "Fixed"})

	page := m.Page()
	// Resolved incidents should NOT appear in active list.
	if len(page.Incidents) != 0 {
		t.Error("resolved incident should not be in active list")
	}
}

func TestManager_AddMaintenance(t *testing.T) {
	m := NewManager()
	m.AddMaintenance(Maintenance{
		ID:       "maint-1",
		Title:    "DB migration",
		StartsAt: time.Now().Add(1 * time.Hour),
		EndsAt:   time.Now().Add(2 * time.Hour),
	})

	page := m.Page()
	if len(page.Maintenance) != 1 {
		t.Errorf("maintenance: got %d", len(page.Maintenance))
	}
}

func TestManager_PastMaintenance(t *testing.T) {
	m := NewManager()
	m.AddMaintenance(Maintenance{
		ID:       "maint-1",
		Title:    "Past event",
		StartsAt: time.Now().Add(-2 * time.Hour),
		EndsAt:   time.Now().Add(-1 * time.Hour),
	})

	page := m.Page()
	if len(page.Maintenance) != 0 {
		t.Error("past maintenance should not appear")
	}
}

func TestManager_RemoveMaintenance(t *testing.T) {
	m := NewManager()
	m.AddMaintenance(Maintenance{ID: "maint-1", Title: "Event", EndsAt: time.Now().Add(1 * time.Hour)})
	m.RemoveMaintenance("maint-1")

	page := m.Page()
	if len(page.Maintenance) != 0 {
		t.Error("removed maintenance should not appear")
	}
}

func TestManager_Handler_Operational(t *testing.T) {
	m := NewManager()
	m.SetComponent(Component{Name: "API", Status: StatusOperational})

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()
	m.Handler()(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d", w.Code)
	}

	var page Page
	json.NewDecoder(w.Body).Decode(&page)
	if page.Status != StatusOperational {
		t.Errorf("body status: got %q", page.Status)
	}
}

func TestManager_Handler_MajorOutage(t *testing.T) {
	m := NewManager()
	m.SetComponent(Component{Name: "DB", Status: StatusMajorOutage})

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()
	m.Handler()(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("major outage: got %d, want 503", w.Code)
	}
}

func TestManager_SortedComponents(t *testing.T) {
	m := NewManager()
	m.SetComponent(Component{Name: "Zulu", Status: StatusOperational})
	m.SetComponent(Component{Name: "Alpha", Status: StatusOperational})

	page := m.Page()
	if page.Components[0].Name != "Alpha" {
		t.Errorf("should be sorted: first is %q", page.Components[0].Name)
	}
}

func TestManager_EmptyPage(t *testing.T) {
	m := NewManager()
	page := m.Page()

	if page.Status != StatusOperational {
		t.Errorf("empty: got %q", page.Status)
	}
	if page.UpdatedAt.IsZero() {
		t.Error("should have timestamp")
	}
}

func TestManager_Headers(t *testing.T) {
	m := NewManager()
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()
	m.Handler()(w, req)

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type: got %q", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control: got %q", cc)
	}
}
