package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

var (
	// ErrEventNotFound is returned when an event is not found.
	ErrEventNotFound = errors.New("outbox: event not found")
	// ErrPublishFailed is returned when event publication fails.
	ErrPublishFailed = errors.New("outbox: publish failed")
)

// EventStatus represents the processing state of an outbox event.
type EventStatus string

const (
	StatusPending   EventStatus = "pending"
	StatusPublished EventStatus = "published"
	StatusFailed    EventStatus = "failed"
)

// Event represents an outbox event to be published.
type Event struct {
	ID          string          `json:"id"`
	AggregateID string          `json:"aggregate_id"`
	EventType   string          `json:"event_type"`
	Payload     json.RawMessage `json:"payload"`
	Status      EventStatus     `json:"status"`
	Attempts    int             `json:"attempts"`
	LastError   string          `json:"last_error,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	PublishedAt *time.Time      `json:"published_at,omitempty"`
}

// Publisher sends events to the message broker (Kafka, etc).
type Publisher interface {
	Publish(ctx context.Context, event Event) error
}

// Store persists outbox events (database implementation).
type Store interface {
	// Save stores an event in the outbox (called within the business transaction).
	Save(ctx context.Context, event Event) error
	// FetchPending returns up to limit pending/failed events ordered by creation time.
	FetchPending(ctx context.Context, limit int) ([]Event, error)
	// MarkPublished marks an event as successfully published.
	MarkPublished(ctx context.Context, id string, publishedAt time.Time) error
	// MarkFailed records a publish failure with the error message.
	MarkFailed(ctx context.Context, id string, err string, attempts int) error
}

// Poller periodically reads pending events and publishes them.
type Poller struct {
	store       Store
	publisher   Publisher
	interval    time.Duration
	batchSize   int
	maxAttempts int
	logger      *slog.Logger

	stop chan struct{}
	done chan struct{}
}

// PollerConfig configures the outbox poller.
type PollerConfig struct {
	// Interval between poll cycles. Default: 1 second.
	Interval time.Duration
	// BatchSize is max events per poll. Default: 100.
	BatchSize int
	// MaxAttempts before giving up on an event. Default: 5.
	MaxAttempts int
	// Logger for structured logging. Default: slog.Default().
	Logger *slog.Logger
}

// NewPoller creates a new outbox poller.
func NewPoller(store Store, publisher Publisher, cfg PollerConfig) *Poller {
	if cfg.Interval <= 0 {
		cfg.Interval = time.Second
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 5
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return &Poller{
		store:       store,
		publisher:   publisher,
		interval:    cfg.Interval,
		batchSize:   cfg.BatchSize,
		maxAttempts: cfg.MaxAttempts,
		logger:      cfg.Logger,
		stop:        make(chan struct{}),
		done:        make(chan struct{}),
	}
}

// Start begins the polling loop in a goroutine.
func (p *Poller) Start() {
	go p.run()
}

// Stop gracefully stops the poller and waits for completion.
func (p *Poller) Stop() {
	close(p.stop)
	<-p.done
}

func (p *Poller) run() {
	defer close(p.done)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stop:
			return
		case <-ticker.C:
			p.poll()
		}
	}
}

func (p *Poller) poll() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	events, err := p.store.FetchPending(ctx, p.batchSize)
	if err != nil {
		p.logger.Error("outbox: fetch pending events", "error", err)
		return
	}

	for _, event := range events {
		if event.Attempts >= p.maxAttempts {
			continue
		}

		select {
		case <-p.stop:
			return
		default:
		}

		if err := p.publisher.Publish(ctx, event); err != nil {
			p.logger.Warn("outbox: publish failed",
				"event_id", event.ID,
				"event_type", event.EventType,
				"attempt", event.Attempts+1,
				"error", err,
			)
			if storeErr := p.store.MarkFailed(ctx, event.ID, err.Error(), event.Attempts+1); storeErr != nil {
				p.logger.Error("outbox: mark failed", "event_id", event.ID, "error", storeErr)
			}
		} else {
			now := time.Now()
			if storeErr := p.store.MarkPublished(ctx, event.ID, now); storeErr != nil {
				p.logger.Error("outbox: mark published", "event_id", event.ID, "error", storeErr)
			}
		}
	}
}

// PollOnce runs a single poll cycle. Useful for testing.
func (p *Poller) PollOnce() {
	p.poll()
}

// MemoryStore is an in-memory outbox store for testing.
type MemoryStore struct {
	mu     sync.RWMutex
	events map[string]*Event
	order  []string
}

// NewMemoryStore creates a new in-memory outbox store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		events: make(map[string]*Event),
	}
}

// Save stores an event.
func (s *MemoryStore) Save(_ context.Context, event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e := event
	s.events[event.ID] = &e
	s.order = append(s.order, event.ID)
	return nil
}

// FetchPending returns pending/failed events in order.
func (s *MemoryStore) FetchPending(_ context.Context, limit int) ([]Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Event
	for _, id := range s.order {
		if len(result) >= limit {
			break
		}
		e := s.events[id]
		if e.Status == StatusPending || e.Status == StatusFailed {
			result = append(result, *e)
		}
	}
	return result, nil
}

// MarkPublished marks an event as published.
func (s *MemoryStore) MarkPublished(_ context.Context, id string, publishedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.events[id]
	if !ok {
		return ErrEventNotFound
	}
	e.Status = StatusPublished
	e.PublishedAt = &publishedAt
	return nil
}

// MarkFailed records a failure.
func (s *MemoryStore) MarkFailed(_ context.Context, id string, errMsg string, attempts int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.events[id]
	if !ok {
		return ErrEventNotFound
	}
	e.Status = StatusFailed
	e.LastError = errMsg
	e.Attempts = attempts
	return nil
}

// Get returns an event by ID.
func (s *MemoryStore) Get(id string) (*Event, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.events[id]
	if !ok {
		return nil, false
	}
	copy := *e
	return &copy, true
}

// Count returns the number of events in the store.
func (s *MemoryStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.events)
}

// NewEvent is a helper to create a new outbox event.
func NewEvent(id, aggregateID, eventType string, payload interface{}) (Event, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return Event{}, fmt.Errorf("outbox: marshal payload: %w", err)
	}

	return Event{
		ID:          id,
		AggregateID: aggregateID,
		EventType:   eventType,
		Payload:     data,
		Status:      StatusPending,
		CreatedAt:   time.Now(),
	}, nil
}
