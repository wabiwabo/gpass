package respwriter

import (
	"net/http"
	"sync"
)

// Capture wraps http.ResponseWriter to capture status code and body size.
type Capture struct {
	http.ResponseWriter
	StatusCode int
	BodySize   int64
	written    bool
}

// NewCapture wraps a ResponseWriter for status/size capture.
func NewCapture(w http.ResponseWriter) *Capture {
	return &Capture{
		ResponseWriter: w,
		StatusCode:     http.StatusOK,
	}
}

// WriteHeader captures the status code.
func (c *Capture) WriteHeader(code int) {
	if !c.written {
		c.StatusCode = code
		c.written = true
	}
	c.ResponseWriter.WriteHeader(code)
}

// Write captures body size and writes data.
func (c *Capture) Write(b []byte) (int, error) {
	if !c.written {
		c.StatusCode = http.StatusOK
		c.written = true
	}
	n, err := c.ResponseWriter.Write(b)
	c.BodySize += int64(n)
	return n, err
}

// Written returns whether WriteHeader or Write has been called.
func (c *Capture) Written() bool {
	return c.written
}

// Unwrap returns the original ResponseWriter (for http.Flusher etc).
func (c *Capture) Unwrap() http.ResponseWriter {
	return c.ResponseWriter
}

// Buffer captures response body in memory for inspection/modification.
type Buffer struct {
	http.ResponseWriter
	StatusCode int
	Body       []byte
	headers    http.Header
	written    bool
}

// NewBuffer creates a buffered response writer.
func NewBuffer(w http.ResponseWriter) *Buffer {
	return &Buffer{
		ResponseWriter: w,
		StatusCode:     http.StatusOK,
		headers:        make(http.Header),
	}
}

// Header returns the header map.
func (b *Buffer) Header() http.Header {
	return b.headers
}

// WriteHeader captures the status code without writing.
func (b *Buffer) WriteHeader(code int) {
	if !b.written {
		b.StatusCode = code
		b.written = true
	}
}

// Write captures body bytes without writing.
func (b *Buffer) Write(data []byte) (int, error) {
	if !b.written {
		b.StatusCode = http.StatusOK
		b.written = true
	}
	b.Body = append(b.Body, data...)
	return len(data), nil
}

// Flush writes the buffered response to the underlying writer.
func (b *Buffer) Flush() {
	// Copy headers.
	for k, vals := range b.headers {
		for _, v := range vals {
			b.ResponseWriter.Header().Add(k, v)
		}
	}
	b.ResponseWriter.WriteHeader(b.StatusCode)
	b.ResponseWriter.Write(b.Body)
}

// Pool provides reusable Capture writers to reduce allocations.
var capturePool = sync.Pool{
	New: func() interface{} {
		return &Capture{}
	},
}

// GetCapture gets a Capture from the pool.
func GetCapture(w http.ResponseWriter) *Capture {
	c := capturePool.Get().(*Capture)
	c.ResponseWriter = w
	c.StatusCode = http.StatusOK
	c.BodySize = 0
	c.written = false
	return c
}

// PutCapture returns a Capture to the pool.
func PutCapture(c *Capture) {
	c.ResponseWriter = nil
	capturePool.Put(c)
}
