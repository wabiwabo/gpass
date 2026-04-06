package result

import (
	"errors"
	"strconv"
	"testing"
)

func TestOk(t *testing.T) {
	r := Ok(42)
	if !r.IsOk() {
		t.Error("should be ok")
	}
	if r.IsErr() {
		t.Error("should not be err")
	}
	if r.Value() != 42 {
		t.Errorf("Value = %d", r.Value())
	}
	if r.Error() != nil {
		t.Error("Error should be nil")
	}
}

func TestErr(t *testing.T) {
	r := Err[int](errors.New("fail"))
	if r.IsOk() {
		t.Error("should not be ok")
	}
	if !r.IsErr() {
		t.Error("should be err")
	}
	if r.Error() == nil {
		t.Error("Error should not be nil")
	}
}

func TestValue_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("should panic on Value() for error result")
		}
	}()
	r := Err[string](errors.New("fail"))
	r.Value()
}

func TestValueOr(t *testing.T) {
	ok := Ok(42)
	if ok.ValueOr(0) != 42 {
		t.Error("should return value")
	}

	err := Err[int](errors.New("fail"))
	if err.ValueOr(99) != 99 {
		t.Error("should return default")
	}
}

func TestUnwrap(t *testing.T) {
	v, err := Ok("hello").Unwrap()
	if err != nil || v != "hello" {
		t.Errorf("Unwrap = (%q, %v)", v, err)
	}

	v, err = Err[string](errors.New("fail")).Unwrap()
	if err == nil {
		t.Error("should have error")
	}
}

func TestMap(t *testing.T) {
	r := Ok(42)
	mapped := Map(r, strconv.Itoa)
	if mapped.Value() != "42" {
		t.Errorf("Map = %q", mapped.Value())
	}

	errResult := Err[int](errors.New("fail"))
	mappedErr := Map(errResult, strconv.Itoa)
	if mappedErr.IsOk() {
		t.Error("Map should propagate error")
	}
}

func TestFlatMap(t *testing.T) {
	r := Ok(42)
	result := FlatMap(r, func(n int) Result[string] {
		return Ok(strconv.Itoa(n))
	})
	if result.Value() != "42" {
		t.Errorf("FlatMap = %q", result.Value())
	}

	errR := Err[int](errors.New("fail"))
	errResult := FlatMap(errR, func(n int) Result[string] {
		return Ok("should not reach")
	})
	if errResult.IsOk() {
		t.Error("FlatMap should propagate error")
	}
}

func TestFrom(t *testing.T) {
	r := From(strconv.Atoi("42"))
	if r.Value() != 42 {
		t.Errorf("From = %d", r.Value())
	}

	r2 := From(strconv.Atoi("abc"))
	if r2.IsOk() {
		t.Error("should be error")
	}
}

func TestMust(t *testing.T) {
	v := Must(Ok(42))
	if v != 42 {
		t.Errorf("Must = %d", v)
	}
}

func TestMust_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("should panic")
		}
	}()
	Must(Err[int](errors.New("fail")))
}

func TestCollect_AllOk(t *testing.T) {
	results := []Result[int]{Ok(1), Ok(2), Ok(3)}
	r := Collect(results)
	if !r.IsOk() {
		t.Fatal("should be ok")
	}
	vals := r.Value()
	if len(vals) != 3 || vals[0] != 1 || vals[2] != 3 {
		t.Errorf("Collect = %v", vals)
	}
}

func TestCollect_WithError(t *testing.T) {
	results := []Result[int]{Ok(1), Err[int](errors.New("fail")), Ok(3)}
	r := Collect(results)
	if r.IsOk() {
		t.Error("should propagate error")
	}
}

func TestCollect_Empty(t *testing.T) {
	r := Collect([]Result[int]{})
	if !r.IsOk() {
		t.Error("empty should be ok")
	}
	if len(r.Value()) != 0 {
		t.Error("should be empty slice")
	}
}
