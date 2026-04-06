package poolmon

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// PoolStats represents database connection pool statistics.
type PoolStats struct {
	Name            string        `json:"name"`
	MaxOpen         int           `json:"max_open"`
	Open            int           `json:"open"`
	InUse           int           `json:"in_use"`
	Idle            int           `json:"idle"`
	WaitCount       int64         `json:"wait_count"`
	WaitDuration    time.Duration `json:"wait_duration_ns"`
	WaitDurationStr string        `json:"wait_duration"`
	MaxIdleClosed   int64         `json:"max_idle_closed"`
	MaxLifetClosed  int64         `json:"max_lifetime_closed"`
	Status          string        `json:"status"` // "healthy", "warning", "critical"
}

// PoolSource represents a monitored database pool.
type PoolSource struct {
	Name string
	DB   *sql.DB
}

// Monitor tracks multiple database pools and provides health endpoints.
type Monitor struct {
	mu      sync.RWMutex
	pools   []PoolSource
	thresholds Thresholds
}

// Thresholds defines warning/critical thresholds for pool health.
type Thresholds struct {
	// WarnUtilization triggers warning when in_use/max_open exceeds this ratio (0-1).
	WarnUtilization float64
	// CritUtilization triggers critical status.
	CritUtilization float64
	// WarnWaitDuration triggers warning when avg wait exceeds this.
	WarnWaitDuration time.Duration
}

// DefaultThresholds returns sensible defaults.
func DefaultThresholds() Thresholds {
	return Thresholds{
		WarnUtilization:  0.7,
		CritUtilization:  0.9,
		WarnWaitDuration: 100 * time.Millisecond,
	}
}

// NewMonitor creates a new pool monitor.
func NewMonitor(thresholds Thresholds) *Monitor {
	return &Monitor{
		thresholds: thresholds,
	}
}

// Register adds a database pool to monitor.
func (m *Monitor) Register(name string, db *sql.DB) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pools = append(m.pools, PoolSource{Name: name, DB: db})
}

// Stats returns current stats for all monitored pools.
func (m *Monitor) Stats() []PoolStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make([]PoolStats, 0, len(m.pools))
	for _, p := range m.pools {
		s := p.DB.Stats()
		ps := PoolStats{
			Name:            p.Name,
			MaxOpen:         s.MaxOpenConnections,
			Open:            s.OpenConnections,
			InUse:           s.InUse,
			Idle:            s.Idle,
			WaitCount:       s.WaitCount,
			WaitDuration:    s.WaitDuration,
			WaitDurationStr: s.WaitDuration.String(),
			MaxIdleClosed:   s.MaxIdleClosed,
			MaxLifetClosed:  s.MaxLifetimeClosed,
			Status:          m.evaluate(s),
		}
		stats = append(stats, ps)
	}
	return stats
}

func (m *Monitor) evaluate(s sql.DBStats) string {
	if s.MaxOpenConnections > 0 {
		utilization := float64(s.InUse) / float64(s.MaxOpenConnections)
		if utilization >= m.thresholds.CritUtilization {
			return "critical"
		}
		if utilization >= m.thresholds.WarnUtilization {
			return "warning"
		}
	}
	if s.WaitCount > 0 && m.thresholds.WarnWaitDuration > 0 {
		avgWait := time.Duration(int64(s.WaitDuration) / s.WaitCount)
		if avgWait >= m.thresholds.WarnWaitDuration {
			return "warning"
		}
	}
	return "healthy"
}

// Handler returns an HTTP handler exposing pool stats as JSON.
func (m *Monitor) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		stats := m.Stats()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(stats)
	}
}

// IsHealthy returns true if all pools are healthy.
func (m *Monitor) IsHealthy() bool {
	for _, s := range m.Stats() {
		if s.Status == "critical" {
			return false
		}
	}
	return true
}
