package hashutil

import (
	"strings"
	"testing"
)

func TestSHA256(t *testing.T) {
	h := SHA256("hello")
	if !strings.HasPrefix(h, "2cf24dba") { t.Errorf("hash = %q", h) }
	if len(h) != 64 { t.Errorf("len = %d", len(h)) }
}

func TestSHA256Bytes(t *testing.T) {
	h := SHA256Bytes([]byte("hello"))
	if h != SHA256("hello") { t.Error("should match") }
}

func TestSHA512(t *testing.T) {
	h := SHA512("hello")
	if len(h) != 128 { t.Errorf("len = %d", len(h)) }
	if h == SHA256("hello") { t.Error("different algorithm") }
}

func TestSHA256Short(t *testing.T) {
	h := SHA256Short("hello", 8)
	if len(h) != 8 { t.Errorf("len = %d", len(h)) }
	if !strings.HasPrefix(SHA256("hello"), h) { t.Error("should be prefix") }
}

func TestSHA256Short_TooLong(t *testing.T) {
	h := SHA256Short("hello", 100)
	if len(h) != 64 { t.Error("should cap at full hash") }
}

func TestCombine(t *testing.T) {
	h1 := Combine("a", "b", "c")
	h2 := Combine("a", "b", "c")
	h3 := Combine("a", "b", "d")
	if h1 != h2 { t.Error("deterministic") }
	if h1 == h3 { t.Error("different input") }
}

func TestDeterministicID(t *testing.T) {
	id := DeterministicID("user", "123")
	if len(id) != 16 { t.Errorf("len = %d", len(id)) }

	id2 := DeterministicID("user", "123")
	if id != id2 { t.Error("deterministic") }

	id3 := DeterministicID("user", "456")
	if id == id3 { t.Error("different input") }
}

func TestEqual(t *testing.T) {
	a := SHA256("test")
	b := SHA256("test")
	c := SHA256("other")
	if !Equal(a, b) { t.Error("same") }
	if Equal(a, c) { t.Error("different") }
	if Equal("short", "longer-string") { t.Error("different length") }
}
