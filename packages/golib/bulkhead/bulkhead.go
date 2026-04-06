package bulkhead

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// ErrBulkheadFull is returned when the bulkhead has no available permits.
	ErrBulkheadFull = errors.New("bulkhead: no permits available")
	// ErrBulkheadTimeout is returned when waiting for a permit times out.
	ErrBulkheadTimeout = errors.New("bulkhead: timeout waiting for permit")
)

// Metrics holds bulkhead usage statistics.
type Metrics struct {
	TotalAccepted  int64
	TotalRejected  int64
	TotalCompleted int64
	TotalFailed    int64
	ActiveCount    int64
	QueuedCount    int64
	MaxConcurrent  int
	MaxQueue       int
}

// Config configures the bulkhead.
type Config struct {
	// Name identifies this bulkhead (for metrics/logging).
	Name string
	// MaxConcurrent is the maximum number of concurrent executions.
	MaxConcurrent int
	// MaxQueue is the maximum number of requests waiting for a permit.
	// Set to 0 to reject immediately when full.
	MaxQueue int
	// QueueTimeout is how long to wait for a permit when queued.
	// Ignored when MaxQueue is 0. Defaults to 1 second.
	QueueTimeout time.Duration
}

// Bulkhead limits concurrent access to a resource using a semaphore pattern.
// It prevents one slow dependency from consuming all available goroutines.
type Bulkhead struct {
	name         string
	sem          chan struct{}
	queue        chan struct{}
	queueTimeout time.Duration

	accepted  atomic.Int64
	rejected  atomic.Int64
	completed atomic.Int64
	failed    atomic.Int64
	active    atomic.Int64
	queued    atomic.Int64
	maxConc   int
	maxQueue  int
}

// New creates a new Bulkhead with the given configuration.
func New(cfg Config) *Bulkhead {
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 10
	}
	if cfg.QueueTimeout <= 0 {
		cfg.QueueTimeout = time.Second
	}

	b := &Bulkhead{
		name:         cfg.Name,
		sem:          make(chan struct{}, cfg.MaxConcurrent),
		queueTimeout: cfg.QueueTimeout,
		maxConc:      cfg.MaxConcurrent,
		maxQueue:     cfg.MaxQueue,
	}

	if cfg.MaxQueue > 0 {
		b.queue = make(chan struct{}, cfg.MaxQueue)
	}

	return b
}

// Execute runs fn within the bulkhead's concurrency limit.
// Returns ErrBulkheadFull if no permits and no queue space, or
// ErrBulkheadTimeout if queued but timed out waiting.
func (b *Bulkhead) Execute(ctx context.Context, fn func(context.Context) error) error {
	// Try to acquire a permit immediately.
	select {
	case b.sem <- struct{}{}:
		b.accepted.Add(1)
		b.active.Add(1)
		err := b.run(ctx, fn)
		return err
	default:
	}

	// No immediate permit available. Try queue if enabled.
	if b.queue == nil {
		b.rejected.Add(1)
		return ErrBulkheadFull
	}

	// Try to enter the queue.
	select {
	case b.queue <- struct{}{}:
		b.queued.Add(1)
	default:
		b.rejected.Add(1)
		return ErrBulkheadFull
	}

	// Wait for a permit with timeout.
	timer := time.NewTimer(b.queueTimeout)
	defer timer.Stop()

	select {
	case b.sem <- struct{}{}:
		b.queued.Add(-1)
		<-b.queue // free queue slot
		b.accepted.Add(1)
		b.active.Add(1)
		return b.run(ctx, fn)
	case <-timer.C:
		b.queued.Add(-1)
		<-b.queue
		b.rejected.Add(1)
		return ErrBulkheadTimeout
	case <-ctx.Done():
		b.queued.Add(-1)
		<-b.queue
		b.rejected.Add(1)
		return ctx.Err()
	}
}

// ExecuteWithResult runs fn and returns a value within the bulkhead's concurrency limit.
func ExecuteWithResult[T any](b *Bulkhead, ctx context.Context, fn func(context.Context) (T, error)) (T, error) {
	var result T
	err := b.Execute(ctx, func(ctx context.Context) error {
		var fnErr error
		result, fnErr = fn(ctx)
		return fnErr
	})
	return result, err
}

func (b *Bulkhead) run(ctx context.Context, fn func(context.Context) error) error {
	defer func() {
		<-b.sem
		b.active.Add(-1)
	}()

	err := fn(ctx)
	if err != nil {
		b.failed.Add(1)
	} else {
		b.completed.Add(1)
	}
	return err
}

// Metrics returns current bulkhead metrics.
func (b *Bulkhead) Metrics() Metrics {
	return Metrics{
		TotalAccepted:  b.accepted.Load(),
		TotalRejected:  b.rejected.Load(),
		TotalCompleted: b.completed.Load(),
		TotalFailed:    b.failed.Load(),
		ActiveCount:    b.active.Load(),
		QueuedCount:    b.queued.Load(),
		MaxConcurrent:  b.maxConc,
		MaxQueue:       b.maxQueue,
	}
}

// Name returns the bulkhead's name.
func (b *Bulkhead) Name() string {
	return b.name
}

// Registry manages named bulkheads for different services/resources.
type Registry struct {
	mu        sync.RWMutex
	bulkheads map[string]*Bulkhead
}

// NewRegistry creates a new bulkhead registry.
func NewRegistry() *Registry {
	return &Registry{
		bulkheads: make(map[string]*Bulkhead),
	}
}

// Register adds a bulkhead to the registry.
func (r *Registry) Register(b *Bulkhead) {
	r.mu.Lock()
	r.bulkheads[b.name] = b
	r.mu.Unlock()
}

// Get retrieves a bulkhead by name.
func (r *Registry) Get(name string) *Bulkhead {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.bulkheads[name]
}

// All returns metrics for all registered bulkheads.
func (r *Registry) All() map[string]Metrics {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]Metrics, len(r.bulkheads))
	for name, b := range r.bulkheads {
		result[name] = b.Metrics()
	}
	return result
}
