package pipeline

import (
	"errors"
	"strings"
	"testing"
)

func TestPipeline_Run(t *testing.T) {
	p := New[string]().
		Add(Map[string](strings.TrimSpace)).
		Add(Map[string](strings.ToLower)).
		Add(Map[string](func(s string) string { return strings.ReplaceAll(s, " ", "_") }))

	result, err := p.Run("  Hello World  ")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result != "hello_world" {
		t.Errorf("result = %q", result)
	}
}

func TestPipeline_RunEmpty(t *testing.T) {
	p := New[int]()
	result, err := p.Run(42)
	if err != nil || result != 42 {
		t.Errorf("empty pipeline = (%d, %v)", result, err)
	}
}

func TestPipeline_Error(t *testing.T) {
	p := New[int]().
		Add(func(n int) (int, error) { return n * 2, nil }).
		Add(func(n int) (int, error) { return 0, errors.New("failed") }).
		Add(func(n int) (int, error) { return n * 3, nil }) // should not run

	_, err := p.Run(5)
	if err == nil {
		t.Error("should propagate error")
	}
}

func TestPipeline_RunAll(t *testing.T) {
	p := New[int]().
		Add(func(n int) (int, error) { return n + 1, nil }).
		Add(func(n int) (int, error) { return 0, errors.New("err1") }).
		Add(func(n int) (int, error) { return 0, errors.New("err2") })

	result, errs := p.RunAll(0)
	if len(errs) != 2 {
		t.Errorf("errs = %d, want 2", len(errs))
	}
	if result != 1 { // only first stage succeeded
		t.Errorf("result = %d, want 1", result)
	}
}

func TestPipeline_Len(t *testing.T) {
	p := New[int]().
		Add(func(n int) (int, error) { return n, nil }).
		Add(func(n int) (int, error) { return n, nil })

	if p.Len() != 2 {
		t.Errorf("Len = %d", p.Len())
	}
}

func TestOf(t *testing.T) {
	p := Of(
		Map[int](func(n int) int { return n + 1 }),
		Map[int](func(n int) int { return n * 2 }),
	)

	result, err := p.Run(5)
	if err != nil {
		t.Fatal(err)
	}
	if result != 12 { // (5+1)*2
		t.Errorf("result = %d, want 12", result)
	}
}

func TestValidate(t *testing.T) {
	p := New[string]().
		Add(Validate[string](func(s string) error {
			if s == "" {
				return errors.New("empty")
			}
			return nil
		})).
		Add(Map[string](strings.ToUpper))

	// Valid input
	result, err := p.Run("hello")
	if err != nil || result != "HELLO" {
		t.Errorf("valid = (%q, %v)", result, err)
	}

	// Invalid input
	_, err = p.Run("")
	if err == nil {
		t.Error("should fail validation")
	}
}

func TestMap_Stage(t *testing.T) {
	stage := Map[int](func(n int) int { return n * 10 })
	result, err := stage(5)
	if err != nil || result != 50 {
		t.Errorf("Map = (%d, %v)", result, err)
	}
}

func TestPipeline_Chaining(t *testing.T) {
	p := New[string]()
	p.Add(Map[string](strings.TrimSpace))
	p.Add(Map[string](strings.ToLower))

	r, _ := p.Run("  TEST  ")
	if r != "test" {
		t.Errorf("result = %q", r)
	}
}
