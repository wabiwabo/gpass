package respwriter

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCapture_DefaultStatus(t *testing.T) {
	w := httptest.NewRecorder()
	c := NewCapture(w)

	c.Write([]byte("hello"))

	if c.StatusCode != http.StatusOK {
		t.Errorf("default status: got %d", c.StatusCode)
	}
}

func TestCapture_ExplicitStatus(t *testing.T) {
	w := httptest.NewRecorder()
	c := NewCapture(w)

	c.WriteHeader(http.StatusNotFound)

	if c.StatusCode != 404 {
		t.Errorf("status: got %d", c.StatusCode)
	}
}

func TestCapture_BodySize(t *testing.T) {
	w := httptest.NewRecorder()
	c := NewCapture(w)

	c.Write([]byte("hello"))
	c.Write([]byte(" world"))

	if c.BodySize != 11 {
		t.Errorf("body size: got %d, want 11", c.BodySize)
	}
}

func TestCapture_Written(t *testing.T) {
	w := httptest.NewRecorder()
	c := NewCapture(w)

	if c.Written() {
		t.Error("should not be written initially")
	}

	c.WriteHeader(200)
	if !c.Written() {
		t.Error("should be written after WriteHeader")
	}
}

func TestCapture_DoubleWriteHeader(t *testing.T) {
	w := httptest.NewRecorder()
	c := NewCapture(w)

	c.WriteHeader(201)
	c.WriteHeader(500) // should be ignored

	if c.StatusCode != 201 {
		t.Errorf("first status should stick: got %d", c.StatusCode)
	}
}

func TestCapture_Unwrap(t *testing.T) {
	w := httptest.NewRecorder()
	c := NewCapture(w)

	if c.Unwrap() != w {
		t.Error("Unwrap should return original writer")
	}
}

func TestBuffer_CapturesBody(t *testing.T) {
	w := httptest.NewRecorder()
	b := NewBuffer(w)

	b.WriteHeader(http.StatusCreated)
	b.Write([]byte(`{"id":"123"}`))

	if b.StatusCode != 201 {
		t.Errorf("status: got %d", b.StatusCode)
	}
	if string(b.Body) != `{"id":"123"}` {
		t.Errorf("body: got %q", string(b.Body))
	}

	// Underlying writer should not have received anything yet.
	if w.Code != 200 { // httptest.Recorder default
		t.Log("buffer should not write to underlying until Flush")
	}
}

func TestBuffer_Flush(t *testing.T) {
	w := httptest.NewRecorder()
	b := NewBuffer(w)

	b.Header().Set("X-Custom", "value")
	b.WriteHeader(http.StatusCreated)
	b.Write([]byte("data"))

	b.Flush()

	if w.Code != 201 {
		t.Errorf("flushed status: got %d", w.Code)
	}
	if w.Body.String() != "data" {
		t.Errorf("flushed body: got %q", w.Body.String())
	}
	if w.Header().Get("X-Custom") != "value" {
		t.Errorf("flushed header: got %q", w.Header().Get("X-Custom"))
	}
}

func TestBuffer_MultipleWrites(t *testing.T) {
	w := httptest.NewRecorder()
	b := NewBuffer(w)

	b.Write([]byte("part1"))
	b.Write([]byte("part2"))

	if string(b.Body) != "part1part2" {
		t.Errorf("body: got %q", string(b.Body))
	}
}

func TestCapturePool(t *testing.T) {
	w := httptest.NewRecorder()
	c := GetCapture(w)

	c.WriteHeader(201)
	c.Write([]byte("test"))

	if c.StatusCode != 201 {
		t.Error("pooled capture should work")
	}
	if c.BodySize != 4 {
		t.Error("body size should be tracked")
	}

	PutCapture(c)

	// Get another one — should be reset.
	c2 := GetCapture(w)
	if c2.StatusCode != 200 {
		t.Errorf("reset status: got %d", c2.StatusCode)
	}
	if c2.BodySize != 0 {
		t.Errorf("reset body size: got %d", c2.BodySize)
	}
	PutCapture(c2)
}
