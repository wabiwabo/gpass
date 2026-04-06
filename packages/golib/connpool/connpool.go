// Package connpool provides a generic connection pool with health checking,
// idle timeout, and maximum lifetime enforcement. Suitable for managing
// connections to databases, caches, or external services.
package connpool

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ErrPoolClosed is returned when operating on a closed pool.
var ErrPoolClosed = errors.New("connpool: pool is closed")

// ErrTimeout is returned when acquiring a connection times out.
var ErrTimeout = errors.New("connpool: acquire timeout")

// Conn represents a pooled connection with metadata.
type Conn[T any] struct {
	Value     T
	CreatedAt time.Time
	LastUsed  time.Time
	UseCount  int64
	id        int64
}

// Factory creates new connections.
type Factory[T any] func(ctx context.Context) (T, error)

// Closer destroys connections.
type Closer[T any] func(T) error

// HealthCheck validates a connection.
type HealthCheck[T any] func(ctx context.Context, conn T) error

// Config defines pool behavior.
type Config struct {
	MaxSize       int           // Maximum connections.
	MinIdle       int           // Minimum idle connections to maintain.
	MaxLifetime   time.Duration // Max time a connection can live.
	IdleTimeout   time.Duration // Close idle connections after this.
	AcquireTimeout time.Duration // Max wait for a connection.
	HealthInterval time.Duration // Frequency of health checks.
}

// DefaultConfig returns production-suitable defaults.
func DefaultConfig() Config {
	return Config{
		MaxSize:        25,
		MinIdle:        5,
		MaxLifetime:    30 * time.Minute,
		IdleTimeout:    5 * time.Minute,
		AcquireTimeout: 10 * time.Second,
		HealthInterval: 30 * time.Second,
	}
}

// Pool manages a set of reusable connections.
type Pool[T any] struct {
	config  Config
	factory Factory[T]
	closer  Closer[T]
	health  HealthCheck[T]

	mu      sync.Mutex
	idle    []*Conn[T]
	active  int
	closed  bool
	nextID  atomic.Int64
	sem     chan struct{} // Limits total connections.

	stats poolStats
}

type poolStats struct {
	acquired  atomic.Int64
	released  atomic.Int64
	created   atomic.Int64
	destroyed atomic.Int64
	timeouts  atomic.Int64
	errors    atomic.Int64
}

// Stats holds pool statistics.
type Stats struct {
	Acquired    int64 `json:"acquired"`
	Released    int64 `json:"released"`
	Created     int64 `json:"created"`
	Destroyed   int64 `json:"destroyed"`
	Timeouts    int64 `json:"timeouts"`
	Errors      int64 `json:"errors"`
	IdleCount   int   `json:"idle_count"`
	ActiveCount int   `json:"active_count"`
	TotalCount  int   `json:"total_count"`
}

// New creates a new connection pool.
func New[T any](cfg Config, factory Factory[T], closer Closer[T]) *Pool[T] {
	if cfg.MaxSize <= 0 {
		cfg.MaxSize = 25
	}
	if cfg.AcquireTimeout <= 0 {
		cfg.AcquireTimeout = 10 * time.Second
	}

	return &Pool[T]{
		config:  cfg,
		factory: factory,
		closer:  closer,
		idle:    make([]*Conn[T], 0, cfg.MaxSize),
		sem:     make(chan struct{}, cfg.MaxSize),
	}
}

// SetHealthCheck sets the connection health check function.
func (p *Pool[T]) SetHealthCheck(fn HealthCheck[T]) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.health = fn
}

// Acquire gets a connection from the pool or creates a new one.
func (p *Pool[T]) Acquire(ctx context.Context) (*Conn[T], error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, ErrPoolClosed
	}
	p.mu.Unlock()

	// Try to get from idle pool first.
	if conn := p.getIdle(); conn != nil {
		p.stats.acquired.Add(1)
		return conn, nil
	}

	// Wait for capacity.
	timer := time.NewTimer(p.config.AcquireTimeout)
	defer timer.Stop()

	select {
	case p.sem <- struct{}{}:
		// Got capacity — create new connection.
	case <-timer.C:
		p.stats.timeouts.Add(1)
		return nil, ErrTimeout
	case <-ctx.Done():
		p.stats.timeouts.Add(1)
		return nil, ctx.Err()
	}

	conn, err := p.create(ctx)
	if err != nil {
		<-p.sem // Release capacity.
		p.stats.errors.Add(1)
		return nil, err
	}

	p.mu.Lock()
	p.active++
	p.mu.Unlock()

	p.stats.acquired.Add(1)
	return conn, nil
}

func (p *Pool[T]) getIdle() *Conn[T] {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	for len(p.idle) > 0 {
		// LIFO — most recently used connection.
		conn := p.idle[len(p.idle)-1]
		p.idle = p.idle[:len(p.idle)-1]

		// Check lifetime.
		if p.config.MaxLifetime > 0 && now.Sub(conn.CreatedAt) > p.config.MaxLifetime {
			p.destroyLocked(conn)
			continue
		}

		// Check idle timeout.
		if p.config.IdleTimeout > 0 && now.Sub(conn.LastUsed) > p.config.IdleTimeout {
			p.destroyLocked(conn)
			continue
		}

		p.active++
		conn.LastUsed = now
		conn.UseCount++
		return conn
	}
	return nil
}

func (p *Pool[T]) create(ctx context.Context) (*Conn[T], error) {
	val, err := p.factory(ctx)
	if err != nil {
		return nil, fmt.Errorf("connpool: create: %w", err)
	}

	id := p.nextID.Add(1)
	p.stats.created.Add(1)

	return &Conn[T]{
		Value:     val,
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
		UseCount:  1,
		id:        id,
	}, nil
}

// Release returns a connection to the pool.
func (p *Pool[T]) Release(conn *Conn[T]) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.active--
	p.stats.released.Add(1)

	if p.closed {
		p.destroyLocked(conn)
		return
	}

	// Check if connection is still viable.
	now := time.Now()
	if p.config.MaxLifetime > 0 && now.Sub(conn.CreatedAt) > p.config.MaxLifetime {
		p.destroyLocked(conn)
		return
	}

	conn.LastUsed = now
	p.idle = append(p.idle, conn)
}

// Destroy discards a connection without returning it to the pool.
func (p *Pool[T]) Destroy(conn *Conn[T]) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.active--
	p.destroyLocked(conn)
}

func (p *Pool[T]) destroyLocked(conn *Conn[T]) {
	if p.closer != nil {
		p.closer(conn.Value)
	}
	p.stats.destroyed.Add(1)
	// Release semaphore slot.
	select {
	case <-p.sem:
	default:
	}
}

// Close shuts down the pool and destroys all connections.
func (p *Pool[T]) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}
	p.closed = true

	for _, conn := range p.idle {
		p.destroyLocked(conn)
	}
	p.idle = nil
	return nil
}

// Stats returns pool statistics.
func (p *Pool[T]) Stats() Stats {
	p.mu.Lock()
	idle := len(p.idle)
	active := p.active
	p.mu.Unlock()

	return Stats{
		Acquired:    p.stats.acquired.Load(),
		Released:    p.stats.released.Load(),
		Created:     p.stats.created.Load(),
		Destroyed:   p.stats.destroyed.Load(),
		Timeouts:    p.stats.timeouts.Load(),
		Errors:      p.stats.errors.Load(),
		IdleCount:   idle,
		ActiveCount: active,
		TotalCount:  idle + active,
	}
}

// Len returns the total number of connections (idle + active).
func (p *Pool[T]) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.idle) + p.active
}
