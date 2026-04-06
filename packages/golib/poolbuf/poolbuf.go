// Package poolbuf provides a sync.Pool-based byte buffer pool.
// Reduces GC pressure by reusing byte buffers for serialization,
// HTTP response writing, and other temporary buffer use cases.
package poolbuf

import (
	"bytes"
	"sync"
)

// Pool is a pool of reusable byte buffers.
type Pool struct {
	pool sync.Pool
}

// New creates a buffer pool with the given initial buffer capacity.
func New(capacity int) *Pool {
	return &Pool{
		pool: sync.Pool{
			New: func() interface{} {
				buf := bytes.NewBuffer(make([]byte, 0, capacity))
				return buf
			},
		},
	}
}

// DefaultPool is a pool with 4KB initial buffers.
var DefaultPool = New(4096)

// Get retrieves a buffer from the pool.
func (p *Pool) Get() *bytes.Buffer {
	return p.pool.Get().(*bytes.Buffer)
}

// Put returns a buffer to the pool after resetting it.
func (p *Pool) Put(buf *bytes.Buffer) {
	buf.Reset()
	p.pool.Put(buf)
}

// Get retrieves a buffer from the default pool.
func Get() *bytes.Buffer {
	return DefaultPool.Get()
}

// Put returns a buffer to the default pool.
func Put(buf *bytes.Buffer) {
	DefaultPool.Put(buf)
}
