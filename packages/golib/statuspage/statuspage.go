// Package statuspage provides a structured service status page that
// aggregates component health, ongoing incidents, and scheduled
// maintenance into a single JSON/HTML-ready response.
package statuspage

import (
	"encoding/json"
	"net/http"
	"sort"
	"sync"
	"time"
)

// Status represents overall or component status.
type Status string

const (
	StatusOperational      Status = "operational"
	StatusDegraded         Status = "degraded_performance"
	StatusPartialOutage    Status = "partial_outage"
	StatusMajorOutage      Status = "major_outage"
	StatusUnderMaintenance Status = "under_maintenance"
)

// Component represents a system component.
type Component struct {
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Status      Status    `json:"status"`
	UpdatedAt   time.Time `json:"updated_at"`
	Group       string    `json:"group,omitempty"`
}

// Incident represents an ongoing or resolved incident.
type Incident struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Status      IncidentStatus  `json:"status"`
	Impact      Status          `json:"impact"`
	CreatedAt   time.Time       `json:"created_at"`
	ResolvedAt  *time.Time      `json:"resolved_at,omitempty"`
	Updates     []IncidentUpdate `json:"updates,omitempty"`
	Components  []string         `json:"components,omitempty"`
}

// IncidentStatus represents the lifecycle of an incident.
type IncidentStatus string

const (
	IncidentInvestigating IncidentStatus = "investigating"
	IncidentIdentified    IncidentStatus = "identified"
	IncidentMonitoring    IncidentStatus = "monitoring"
	IncidentResolved      IncidentStatus = "resolved"
)

// IncidentUpdate is a timestamped update to an incident.
type IncidentUpdate struct {
	Status    IncidentStatus `json:"status"`
	Message   string         `json:"message"`
	CreatedAt time.Time      `json:"created_at"`
}

// Maintenance represents a scheduled maintenance window.
type Maintenance struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
	Components  []string  `json:"components,omitempty"`
}

// Page holds the complete status page data.
type Page struct {
	Status      Status        `json:"status"`
	Components  []Component   `json:"components"`
	Incidents   []Incident    `json:"incidents"`
	Maintenance []Maintenance `json:"scheduled_maintenance"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// Manager manages the status page state.
type Manager struct {
	mu          sync.RWMutex
	components  map[string]*Component
	incidents   map[string]*Incident
	maintenance map[string]*Maintenance
}

// NewManager creates a new status page manager.
func NewManager() *Manager {
	return &Manager{
		components:  make(map[string]*Component),
		incidents:   make(map[string]*Incident),
		maintenance: make(map[string]*Maintenance),
	}
}

// SetComponent adds or updates a component.
func (m *Manager) SetComponent(c Component) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c.UpdatedAt = time.Now()
	m.components[c.Name] = &c
}

// UpdateComponentStatus changes a component's status.
func (m *Manager) UpdateComponentStatus(name string, status Status) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.components[name]; ok {
		c.Status = status
		c.UpdatedAt = time.Now()
	}
}

// AddIncident adds a new incident.
func (m *Manager) AddIncident(inc Incident) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if inc.CreatedAt.IsZero() {
		inc.CreatedAt = time.Now()
	}
	m.incidents[inc.ID] = &inc
}

// UpdateIncident adds an update to an existing incident.
func (m *Manager) UpdateIncident(id string, update IncidentUpdate) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if inc, ok := m.incidents[id]; ok {
		if update.CreatedAt.IsZero() {
			update.CreatedAt = time.Now()
		}
		inc.Status = update.Status
		inc.Updates = append(inc.Updates, update)
		if update.Status == IncidentResolved {
			now := time.Now()
			inc.ResolvedAt = &now
		}
	}
}

// AddMaintenance schedules a maintenance window.
func (m *Manager) AddMaintenance(mt Maintenance) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maintenance[mt.ID] = &mt
}

// RemoveMaintenance removes a scheduled maintenance.
func (m *Manager) RemoveMaintenance(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.maintenance, id)
}

// Page generates the current status page.
func (m *Manager) Page() Page {
	m.mu.RLock()
	defer m.mu.RUnlock()

	page := Page{
		Status:    StatusOperational,
		UpdatedAt: time.Now(),
	}

	// Collect components.
	for _, c := range m.components {
		page.Components = append(page.Components, *c)
		page.Status = worstStatus(page.Status, c.Status)
	}
	sort.Slice(page.Components, func(i, j int) bool {
		return page.Components[i].Name < page.Components[j].Name
	})

	// Collect active incidents.
	for _, inc := range m.incidents {
		if inc.Status != IncidentResolved {
			page.Incidents = append(page.Incidents, *inc)
		}
	}
	sort.Slice(page.Incidents, func(i, j int) bool {
		return page.Incidents[i].CreatedAt.After(page.Incidents[j].CreatedAt)
	})

	// Collect upcoming maintenance.
	now := time.Now()
	for _, mt := range m.maintenance {
		if mt.EndsAt.After(now) {
			page.Maintenance = append(page.Maintenance, *mt)
		}
	}
	sort.Slice(page.Maintenance, func(i, j int) bool {
		return page.Maintenance[i].StartsAt.Before(page.Maintenance[j].StartsAt)
	})

	return page
}

// Handler returns an HTTP handler serving the status page as JSON.
func (m *Manager) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page := m.Page()

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")

		status := http.StatusOK
		if page.Status == StatusMajorOutage {
			status = http.StatusServiceUnavailable
		}
		w.WriteHeader(status)

		json.NewEncoder(w).Encode(page)
	}
}

// ComponentCount returns the number of tracked components.
func (m *Manager) ComponentCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.components)
}

func worstStatus(a, b Status) Status {
	order := map[Status]int{
		StatusOperational:      0,
		StatusDegraded:         1,
		StatusPartialOutage:    2,
		StatusUnderMaintenance: 3,
		StatusMajorOutage:      4,
	}
	if order[b] > order[a] {
		return b
	}
	return a
}
