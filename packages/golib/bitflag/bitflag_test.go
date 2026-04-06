package bitflag

import "testing"

const (
	FlagRead    Flags = 1 << 0
	FlagWrite   Flags = 1 << 1
	FlagExecute Flags = 1 << 2
	FlagAdmin   Flags = 1 << 3
)

func TestSet(t *testing.T) {
	f := Flags(0).Set(FlagRead).Set(FlagWrite)
	if !f.Has(FlagRead) || !f.Has(FlagWrite) {
		t.Error("should have read and write")
	}
	if f.Has(FlagExecute) {
		t.Error("should not have execute")
	}
}

func TestClear(t *testing.T) {
	f := FlagRead | FlagWrite
	f = f.Clear(FlagWrite)
	if f.Has(FlagWrite) {
		t.Error("write should be cleared")
	}
	if !f.Has(FlagRead) {
		t.Error("read should remain")
	}
}

func TestToggle(t *testing.T) {
	f := FlagRead
	f = f.Toggle(FlagRead)
	if f.Has(FlagRead) {
		t.Error("read should be off")
	}
	f = f.Toggle(FlagRead)
	if !f.Has(FlagRead) {
		t.Error("read should be on")
	}
}

func TestHasAny(t *testing.T) {
	f := FlagRead | FlagWrite
	if !f.HasAny(FlagRead | FlagExecute) {
		t.Error("should match read")
	}
	if f.HasAny(FlagExecute | FlagAdmin) {
		t.Error("should not match")
	}
}

func TestHasAll(t *testing.T) {
	f := FlagRead | FlagWrite | FlagExecute
	if !f.HasAll(FlagRead | FlagWrite) {
		t.Error("should have both")
	}
	if f.HasAll(FlagRead | FlagAdmin) {
		t.Error("should not have admin")
	}
}

func TestIsZero(t *testing.T) {
	if !Flags(0).IsZero() {
		t.Error("0 should be zero")
	}
	if FlagRead.IsZero() {
		t.Error("read should not be zero")
	}
}

func TestCount(t *testing.T) {
	f := FlagRead | FlagWrite | FlagAdmin
	if f.Count() != 3 {
		t.Errorf("Count = %d, want 3", f.Count())
	}
	if Flags(0).Count() != 0 {
		t.Error("0 should have count 0")
	}
}

func TestString(t *testing.T) {
	if FlagRead.String() != "1" {
		t.Errorf("1 = %q", FlagRead.String())
	}
	if (FlagRead | FlagWrite).String() != "11" {
		t.Errorf("3 = %q", (FlagRead | FlagWrite).String())
	}
	if Flags(0).String() != "0" {
		t.Errorf("0 = %q", Flags(0).String())
	}
}

func TestNamed(t *testing.T) {
	n := NewNamed()
	n.Define("read", FlagRead)
	n.Define("write", FlagWrite)
	n.Define("admin", FlagAdmin)

	f, ok := n.Get("read")
	if !ok || f != FlagRead {
		t.Error("should find read")
	}

	_, ok = n.Get("missing")
	if ok {
		t.Error("should not find missing")
	}
}

func TestNamed_Names(t *testing.T) {
	n := NewNamed()
	n.Define("read", FlagRead)
	n.Define("write", FlagWrite)
	n.Define("admin", FlagAdmin)

	names := n.Names(FlagRead | FlagAdmin)
	if len(names) != 2 {
		t.Errorf("names = %d", len(names))
	}
}
