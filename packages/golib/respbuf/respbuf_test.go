package respbuf

import (
	"net/http/httptest"
	"testing"
)

func TestNew(t *testing.T) {
	rr := httptest.NewRecorder()
	w := New(rr)
	if w.Status() != 200 {
		t.Errorf("default status: got %d, want 200", w.Status())
	}
	if w.Written() {
		t.Error("should not be written yet")
	}
	if w.Size() != 0 {
		t.Errorf("size: got %d, want 0", w.Size())
	}
}

func TestWrite(t *testing.T) {
	rr := httptest.NewRecorder()
	w := New(rr)
	n, err := w.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("write error: %v", err)
	}
	if n != 5 {
		t.Errorf("wrote %d bytes, want 5", n)
	}
	if w.BodyString() != "hello" {
		t.Errorf("body: got %q", w.BodyString())
	}
	if w.Size() != 5 {
		t.Errorf("size: got %d, want 5", w.Size())
	}
	if !w.Written() {
		t.Error("should be written after Write")
	}
}

func TestWriteHeader(t *testing.T) {
	rr := httptest.NewRecorder()
	w := New(rr)
	w.WriteHeader(201)
	if w.Status() != 201 {
		t.Errorf("status: got %d, want 201", w.Status())
	}
}

func TestWriteHeaderOnce(t *testing.T) {
	rr := httptest.NewRecorder()
	w := New(rr)
	w.WriteHeader(201)
	w.WriteHeader(500) // should be ignored
	if w.Status() != 201 {
		t.Errorf("status: got %d, want 201", w.Status())
	}
}

func TestBody(t *testing.T) {
	rr := httptest.NewRecorder()
	w := New(rr)
	w.Write([]byte("test"))
	if len(w.Body()) != 4 {
		t.Errorf("Body() length: got %d, want 4", len(w.Body()))
	}
}

func TestFlush(t *testing.T) {
	rr := httptest.NewRecorder()
	w := New(rr)
	w.WriteHeader(201)
	w.Write([]byte("created"))
	w.Flush()

	if rr.Code != 201 {
		t.Errorf("flushed status: got %d, want 201", rr.Code)
	}
	if rr.Body.String() != "created" {
		t.Errorf("flushed body: got %q", rr.Body.String())
	}
}

func TestFlushNotLeaking(t *testing.T) {
	rr := httptest.NewRecorder()
	w := New(rr)
	w.Write([]byte("buffered"))

	// Before flush, underlying writer should have no body
	if rr.Body.Len() != 0 {
		t.Error("body should be buffered, not flushed yet")
	}

	w.Flush()
	if rr.Body.String() != "buffered" {
		t.Errorf("after flush: got %q", rr.Body.String())
	}
}

func TestReset(t *testing.T) {
	rr := httptest.NewRecorder()
	w := New(rr)
	w.WriteHeader(404)
	w.Write([]byte("not found"))
	w.Reset()

	if w.Status() != 200 {
		t.Errorf("status after reset: got %d", w.Status())
	}
	if w.Size() != 0 {
		t.Errorf("size after reset: got %d", w.Size())
	}
	if w.Written() {
		t.Error("should not be written after reset")
	}
}

func TestMultipleWrites(t *testing.T) {
	rr := httptest.NewRecorder()
	w := New(rr)
	w.Write([]byte("hello"))
	w.Write([]byte(" "))
	w.Write([]byte("world"))

	if w.BodyString() != "hello world" {
		t.Errorf("body: got %q", w.BodyString())
	}
	if w.Size() != 11 {
		t.Errorf("size: got %d, want 11", w.Size())
	}
}

func TestHeaders(t *testing.T) {
	rr := httptest.NewRecorder()
	w := New(rr)
	w.Header().Set("X-Custom", "test")
	w.WriteHeader(200)
	w.Write([]byte("ok"))
	w.Flush()

	if rr.Header().Get("X-Custom") != "test" {
		t.Error("custom header should be preserved through flush")
	}
}
