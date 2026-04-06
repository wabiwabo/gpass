package dlq

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	StatusPending   = "PENDING"
	StatusReviewed  = "REVIEWED"
	StatusReplayed  = "REPLAYED"
	StatusDiscarded = "DISCARDED"
)

// Entry represents a failed item in the dead letter queue.
type Entry struct {
	ID         string     `json:"id"`
	OriginalID string     `json:"original_id"`
	Type       string     `json:"type"`    // "webhook", "event", "notification"
	Payload    []byte     `json:"payload"` // raw payload
	Error      string     `json:"error"`   // why it failed
	Attempts   int        `json:"attempts"`
	Source     string     `json:"source"` // originating service
	CreatedAt  time.Time  `json:"created_at"`
	ReviewedAt *time.Time `json:"reviewed_at,omitempty"`
	ReplayedAt *time.Time `json:"replayed_at,omitempty"`
	Status     string     `json:"status"`
}

// Queue is a dead letter queue for failed operations.
type Queue struct {
	entries map[string]*Entry
	mu      sync.RWMutex
	seq     int
}

// New creates a new dead letter queue.
func New() *Queue {
	return &Queue{
		entries: make(map[string]*Entry),
	}
}

// Add adds a failed item to the DLQ and returns the assigned entry ID.
func (q *Queue) Add(entry Entry) string {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.seq++
	entry.ID = fmt.Sprintf("dlq-%d", q.seq)
	if entry.Status == "" {
		entry.Status = StatusPending
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	q.entries[entry.ID] = &entry
	return entry.ID
}

// Get retrieves an entry by ID.
func (q *Queue) Get(id string) (*Entry, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	e, ok := q.entries[id]
	if !ok {
		return nil, fmt.Errorf("dlq: entry %q not found", id)
	}
	return e, nil
}

// List returns entries filtered by status and/or type.
// Empty string for status or entryType means no filter.
// limit <= 0 means no limit.
func (q *Queue) List(status, entryType string, limit int) []*Entry {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var result []*Entry
	for _, e := range q.entries {
		if status != "" && e.Status != status {
			continue
		}
		if entryType != "" && e.Type != entryType {
			continue
		}
		result = append(result, e)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// MarkReviewed marks an entry as reviewed by an operator.
func (q *Queue) MarkReviewed(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	e, ok := q.entries[id]
	if !ok {
		return fmt.Errorf("dlq: entry %q not found", id)
	}
	now := time.Now()
	e.Status = StatusReviewed
	e.ReviewedAt = &now
	return nil
}

// Replay marks an entry for replay.
func (q *Queue) Replay(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	e, ok := q.entries[id]
	if !ok {
		return fmt.Errorf("dlq: entry %q not found", id)
	}
	now := time.Now()
	e.Status = StatusReplayed
	e.ReplayedAt = &now
	return nil
}

// Discard marks an entry as discarded (won't be retried).
func (q *Queue) Discard(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	e, ok := q.entries[id]
	if !ok {
		return fmt.Errorf("dlq: entry %q not found", id)
	}
	e.Status = StatusDiscarded
	return nil
}

// Count returns count by status.
func (q *Queue) Count() map[string]int {
	q.mu.RLock()
	defer q.mu.RUnlock()

	counts := make(map[string]int)
	for _, e := range q.entries {
		counts[e.Status]++
	}
	return counts
}

// Handler returns HTTP handlers for DLQ management.
//
//	GET  /internal/dlq            - list entries
//	GET  /internal/dlq/stats      - count by status
//	GET  /internal/dlq/{id}       - get entry
//	POST /internal/dlq/{id}/review  - mark reviewed
//	POST /internal/dlq/{id}/replay  - mark for replay
//	POST /internal/dlq/{id}/discard - discard
func (q *Queue) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /internal/dlq", func(w http.ResponseWriter, r *http.Request) {
		status := r.URL.Query().Get("status")
		entryType := r.URL.Query().Get("type")
		entries := q.List(status, entryType, 0)
		if entries == nil {
			entries = []*Entry{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(entries)
	})

	mux.HandleFunc("GET /internal/dlq/stats", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(q.Count())
	})

	mux.HandleFunc("GET /internal/dlq/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		entry, err := q.Get(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(entry)
	})

	mux.HandleFunc("POST /internal/dlq/{id}/review", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := q.MarkReviewed(id); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "reviewed"})
	})

	mux.HandleFunc("POST /internal/dlq/{id}/replay", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := q.Replay(id); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "replayed"})
	})

	mux.HandleFunc("POST /internal/dlq/{id}/discard", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := q.Discard(id); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "discarded"})
	})

	// Handle the trailing slash variant
	mux.HandleFunc("GET /internal/dlq/", func(w http.ResponseWriter, r *http.Request) {
		// Strip trailing slash and extract potential ID
		path := strings.TrimPrefix(r.URL.Path, "/internal/dlq/")
		if path == "" || path == "stats" {
			// Redirect to the right handler
			if path == "stats" {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(q.Count())
				return
			}
			entries := q.List("", "", 0)
			if entries == nil {
				entries = []*Entry{}
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(entries)
			return
		}
	})

	return mux
}
