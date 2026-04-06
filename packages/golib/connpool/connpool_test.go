package connpool

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type testConn struct {
	id     int64
	closed bool
}

var nextConnID atomic.Int64

func testFactory(ctx context.Context) (*testConn, error) {
	return &testConn{id: nextConnID.Add(1)}, nil
}

func testCloser(c *testConn) error {
	c.closed = true
	return nil
}

func TestPool_AcquireRelease(t *testing.T) {
	pool := New[*testConn](Config{MaxSize: 5, AcquireTimeout: 1 * time.Second}, testFactory, testCloser)
	defer pool.Close()

	conn, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if conn.Value == nil {
		t.Error("connection should not be nil")
	}

	pool.Release(conn)

	stats := pool.Stats()
	if stats.Created != 1 {
		t.Errorf("created: got %d", stats.Created)
	}
	if stats.IdleCount != 1 {
		t.Errorf("idle: got %d", stats.IdleCount)
	}
}

func TestPool_ReuseConnection(t *testing.T) {
	pool := New[*testConn](Config{MaxSize: 5, AcquireTimeout: 1 * time.Second}, testFactory, testCloser)
	defer pool.Close()

	conn1, _ := pool.Acquire(context.Background())
	id1 := conn1.Value.id
	pool.Release(conn1)

	conn2, _ := pool.Acquire(context.Background())
	id2 := conn2.Value.id
	pool.Release(conn2)

	if id1 != id2 {
		t.Error("should reuse connection from idle pool")
	}
	if pool.Stats().Created != 1 {
		t.Error("should only create one connection")
	}
}

func TestPool_MaxSize(t *testing.T) {
	pool := New[*testConn](Config{MaxSize: 2, AcquireTimeout: 50 * time.Millisecond}, testFactory, testCloser)
	defer pool.Close()

	c1, _ := pool.Acquire(context.Background())
	c2, _ := pool.Acquire(context.Background())

	// Third should timeout.
	_, err := pool.Acquire(context.Background())
	if err != ErrTimeout {
		t.Errorf("should timeout: got %v", err)
	}

	pool.Release(c1)
	pool.Release(c2)
}

func TestPool_Destroy(t *testing.T) {
	pool := New[*testConn](Config{MaxSize: 5, AcquireTimeout: 1 * time.Second}, testFactory, testCloser)
	defer pool.Close()

	conn, _ := pool.Acquire(context.Background())
	pool.Destroy(conn)

	if !conn.Value.closed {
		t.Error("destroyed connection should be closed")
	}
	if pool.Stats().Destroyed != 1 {
		t.Errorf("destroyed: got %d", pool.Stats().Destroyed)
	}
}

func TestPool_Close(t *testing.T) {
	pool := New[*testConn](Config{MaxSize: 5, AcquireTimeout: 1 * time.Second}, testFactory, testCloser)

	conn, _ := pool.Acquire(context.Background())
	pool.Release(conn)

	pool.Close()

	_, err := pool.Acquire(context.Background())
	if err != ErrPoolClosed {
		t.Errorf("should return closed error: got %v", err)
	}
}

func TestPool_CloseIdempotent(t *testing.T) {
	pool := New[*testConn](Config{MaxSize: 5, AcquireTimeout: 1 * time.Second}, testFactory, testCloser)
	pool.Close()
	pool.Close() // Should not panic.
}

func TestPool_MaxLifetime(t *testing.T) {
	pool := New[*testConn](Config{
		MaxSize:        5,
		MaxLifetime:    20 * time.Millisecond,
		AcquireTimeout: 1 * time.Second,
	}, testFactory, testCloser)
	defer pool.Close()

	conn, _ := pool.Acquire(context.Background())
	time.Sleep(30 * time.Millisecond)
	pool.Release(conn) // Connection expired, should be destroyed on release.

	// Next acquire should create new connection.
	conn2, _ := pool.Acquire(context.Background())
	if conn2.Value.id == conn.Value.id {
		t.Error("expired connection should not be reused")
	}
	pool.Release(conn2)
}

func TestPool_IdleTimeout(t *testing.T) {
	pool := New[*testConn](Config{
		MaxSize:        5,
		IdleTimeout:    20 * time.Millisecond,
		AcquireTimeout: 1 * time.Second,
	}, testFactory, testCloser)
	defer pool.Close()

	conn, _ := pool.Acquire(context.Background())
	pool.Release(conn)

	time.Sleep(30 * time.Millisecond)

	// Idle connection should be evicted on next acquire.
	conn2, _ := pool.Acquire(context.Background())
	if conn2.Value.id == conn.Value.id {
		t.Error("idle-expired connection should not be reused")
	}
	pool.Release(conn2)
}

func TestPool_ContextCancellation(t *testing.T) {
	pool := New[*testConn](Config{MaxSize: 1, AcquireTimeout: 5 * time.Second}, testFactory, testCloser)
	defer pool.Close()

	c1, _ := pool.Acquire(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := pool.Acquire(ctx)
	if err == nil {
		t.Error("should fail on context cancellation")
	}

	pool.Release(c1)
}

func TestPool_ConcurrentAcquireRelease(t *testing.T) {
	pool := New[*testConn](Config{MaxSize: 10, AcquireTimeout: 1 * time.Second}, testFactory, testCloser)
	defer pool.Close()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := pool.Acquire(context.Background())
			if err != nil {
				return
			}
			time.Sleep(time.Millisecond)
			pool.Release(conn)
		}()
	}
	wg.Wait()

	stats := pool.Stats()
	if stats.ActiveCount != 0 {
		t.Errorf("active after all done: got %d", stats.ActiveCount)
	}
}

func TestPool_Stats(t *testing.T) {
	pool := New[*testConn](Config{MaxSize: 5, AcquireTimeout: 1 * time.Second}, testFactory, testCloser)
	defer pool.Close()

	c1, _ := pool.Acquire(context.Background())
	c2, _ := pool.Acquire(context.Background())
	pool.Release(c1)
	pool.Destroy(c2)

	stats := pool.Stats()
	if stats.Acquired != 2 {
		t.Errorf("acquired: got %d", stats.Acquired)
	}
	if stats.Released != 1 {
		t.Errorf("released: got %d", stats.Released)
	}
	if stats.Destroyed != 1 {
		t.Errorf("destroyed: got %d", stats.Destroyed)
	}
}

func TestPool_Len(t *testing.T) {
	pool := New[*testConn](Config{MaxSize: 5, AcquireTimeout: 1 * time.Second}, testFactory, testCloser)
	defer pool.Close()

	if pool.Len() != 0 {
		t.Errorf("initial len: got %d", pool.Len())
	}

	c1, _ := pool.Acquire(context.Background())
	if pool.Len() != 1 {
		t.Errorf("active len: got %d", pool.Len())
	}

	pool.Release(c1)
	if pool.Len() != 1 { // Now idle.
		t.Errorf("idle len: got %d", pool.Len())
	}
}

func TestPool_DefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxSize != 25 {
		t.Errorf("max size: got %d", cfg.MaxSize)
	}
	if cfg.MinIdle != 5 {
		t.Errorf("min idle: got %d", cfg.MinIdle)
	}
}

func TestPool_ReleaseAfterClose(t *testing.T) {
	pool := New[*testConn](Config{MaxSize: 5, AcquireTimeout: 1 * time.Second}, testFactory, testCloser)
	conn, _ := pool.Acquire(context.Background())
	pool.Close()
	pool.Release(conn) // Should destroy, not return to idle.

	if !conn.Value.closed {
		t.Error("connection released after close should be destroyed")
	}
}

func TestPool_UseCountIncrement(t *testing.T) {
	pool := New[*testConn](Config{MaxSize: 5, AcquireTimeout: 1 * time.Second}, testFactory, testCloser)
	defer pool.Close()

	conn, _ := pool.Acquire(context.Background())
	if conn.UseCount != 1 {
		t.Errorf("first use: got %d", conn.UseCount)
	}
	pool.Release(conn)

	conn2, _ := pool.Acquire(context.Background())
	if conn2.UseCount != 2 {
		t.Errorf("second use: got %d", conn2.UseCount)
	}
	pool.Release(conn2)
}
