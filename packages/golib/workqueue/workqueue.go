package workqueue

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Priority levels for work items.
type Priority int

const (
	PriorityLow    Priority = 0
	PriorityNormal Priority = 1
	PriorityHigh   Priority = 2
)

// Item represents a unit of work in the queue.
type Item struct {
	ID       string
	Priority Priority
	Payload  interface{}
	EnqueuedAt time.Time
}

// Handler processes work items.
type Handler func(ctx context.Context, item Item) error

// Config configures the work queue.
type Config struct {
	// MaxSize is the maximum queue size. 0 means unbounded.
	MaxSize int
	// Workers is the number of concurrent workers. Default: 1.
	Workers int
	// RateLimit limits items processed per second. 0 means no limit.
	RateLimit float64
}

// Queue is a priority-aware work queue with backpressure.
type Queue struct {
	high    chan Item
	normal  chan Item
	low     chan Item
	handler Handler
	workers int
	rate    float64

	processing atomic.Int64
	processed  atomic.Int64
	dropped    atomic.Int64
	errors     atomic.Int64

	stop chan struct{}
	done chan struct{}
	once sync.Once
}

// New creates a new work queue.
func New(cfg Config, handler Handler) *Queue {
	if cfg.Workers <= 0 {
		cfg.Workers = 1
	}
	size := cfg.MaxSize
	if size <= 0 {
		size = 1000
	}

	return &Queue{
		high:    make(chan Item, size),
		normal:  make(chan Item, size),
		low:     make(chan Item, size),
		handler: handler,
		workers: cfg.Workers,
		rate:    cfg.RateLimit,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
}

// Enqueue adds an item to the queue. Returns false if the queue is full (backpressure).
func (q *Queue) Enqueue(item Item) bool {
	item.EnqueuedAt = time.Now()

	var ch chan Item
	switch item.Priority {
	case PriorityHigh:
		ch = q.high
	case PriorityLow:
		ch = q.low
	default:
		ch = q.normal
	}

	select {
	case ch <- item:
		return true
	default:
		q.dropped.Add(1)
		return false
	}
}

// Start begins processing items with the configured number of workers.
func (q *Queue) Start() {
	go func() {
		var wg sync.WaitGroup
		for i := 0; i < q.workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				q.worker()
			}()
		}
		wg.Wait()
		close(q.done)
	}()
}

// Stop gracefully stops the queue and waits for workers to finish.
func (q *Queue) Stop() {
	q.once.Do(func() {
		close(q.stop)
	})
	<-q.done
}

// Drain stops accepting new items and processes remaining items.
func (q *Queue) Drain(timeout time.Duration) {
	q.once.Do(func() {
		close(q.stop)
	})

	select {
	case <-q.done:
	case <-time.After(timeout):
	}
}

func (q *Queue) worker() {
	var limiter <-chan time.Time
	if q.rate > 0 {
		ticker := time.NewTicker(time.Duration(float64(time.Second) / q.rate))
		defer ticker.Stop()
		limiter = ticker.C
	}

	for {
		// Rate limiting.
		if limiter != nil {
			select {
			case <-q.stop:
				q.drainRemaining()
				return
			case <-limiter:
			}
		}

		// Priority selection: high > normal > low.
		select {
		case item := <-q.high:
			q.process(item)
		default:
			select {
			case item := <-q.high:
				q.process(item)
			case item := <-q.normal:
				q.process(item)
			default:
				select {
				case item := <-q.high:
					q.process(item)
				case item := <-q.normal:
					q.process(item)
				case item := <-q.low:
					q.process(item)
				case <-q.stop:
					q.drainRemaining()
					return
				}
			}
		}
	}
}

func (q *Queue) process(item Item) {
	q.processing.Add(1)
	defer q.processing.Add(-1)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := q.handler(ctx, item); err != nil {
		q.errors.Add(1)
	}
	q.processed.Add(1)
}

func (q *Queue) drainRemaining() {
	for {
		select {
		case item := <-q.high:
			q.process(item)
		case item := <-q.normal:
			q.process(item)
		case item := <-q.low:
			q.process(item)
		default:
			return
		}
	}
}

// Stats returns current queue statistics.
type Stats struct {
	HighPending   int   `json:"high_pending"`
	NormalPending int   `json:"normal_pending"`
	LowPending    int   `json:"low_pending"`
	Processing    int64 `json:"processing"`
	Processed     int64 `json:"processed"`
	Dropped       int64 `json:"dropped"`
	Errors        int64 `json:"errors"`
}

// Stats returns current queue statistics.
func (q *Queue) Stats() Stats {
	return Stats{
		HighPending:   len(q.high),
		NormalPending: len(q.normal),
		LowPending:    len(q.low),
		Processing:    q.processing.Load(),
		Processed:     q.processed.Load(),
		Dropped:       q.dropped.Load(),
		Errors:        q.errors.Load(),
	}
}

// Len returns total pending items across all priorities.
func (q *Queue) Len() int {
	return len(q.high) + len(q.normal) + len(q.low)
}
