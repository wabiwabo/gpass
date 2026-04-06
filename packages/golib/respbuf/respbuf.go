// Package respbuf provides a buffered HTTP response writer.
// Captures response data (status, headers, body) before flushing,
// enabling inspection and modification by middleware.
package respbuf

import (
	"bytes"
	"net/http"
)

// Writer is a buffered response writer.
type Writer struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
	wrote  bool
}

// New wraps an http.ResponseWriter with buffering.
func New(w http.ResponseWriter) *Writer {
	return &Writer{ResponseWriter: w, status: http.StatusOK}
}

// WriteHeader captures the status code.
func (w *Writer) WriteHeader(code int) {
	if !w.wrote {
		w.status = code
		w.wrote = true
	}
}

// Write captures the body data.
func (w *Writer) Write(data []byte) (int, error) {
	if !w.wrote {
		w.wrote = true
	}
	return w.body.Write(data)
}

// Status returns the captured status code.
func (w *Writer) Status() int {
	return w.status
}

// Body returns the captured body bytes.
func (w *Writer) Body() []byte {
	return w.body.Bytes()
}

// BodyString returns the captured body as string.
func (w *Writer) BodyString() string {
	return w.body.String()
}

// Size returns the body size in bytes.
func (w *Writer) Size() int {
	return w.body.Len()
}

// Written returns true if WriteHeader or Write was called.
func (w *Writer) Written() bool {
	return w.wrote
}

// Flush writes the captured response to the underlying writer.
func (w *Writer) Flush() {
	w.ResponseWriter.WriteHeader(w.status)
	w.ResponseWriter.Write(w.body.Bytes())
}

// Reset clears the buffer and status.
func (w *Writer) Reset() {
	w.body.Reset()
	w.status = http.StatusOK
	w.wrote = false
}
