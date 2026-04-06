// Package pipeline provides a generic pipeline for sequential
// data transformation. Each stage transforms the input and passes
// to the next stage. Supports error short-circuiting.
package pipeline

// Stage is a transformation function.
type Stage[T any] func(T) (T, error)

// Pipeline chains stages sequentially.
type Pipeline[T any] struct {
	stages []Stage[T]
}

// New creates an empty pipeline.
func New[T any]() *Pipeline[T] {
	return &Pipeline[T]{}
}

// Add appends a stage to the pipeline.
func (p *Pipeline[T]) Add(stage Stage[T]) *Pipeline[T] {
	p.stages = append(p.stages, stage)
	return p
}

// Run executes all stages in order. Stops on first error.
func (p *Pipeline[T]) Run(input T) (T, error) {
	current := input
	for _, stage := range p.stages {
		result, err := stage(current)
		if err != nil {
			return current, err
		}
		current = result
	}
	return current, nil
}

// Len returns the number of stages.
func (p *Pipeline[T]) Len() int {
	return len(p.stages)
}

// RunAll executes all stages, collecting all errors.
func (p *Pipeline[T]) RunAll(input T) (T, []error) {
	current := input
	var errs []error
	for _, stage := range p.stages {
		result, err := stage(current)
		if err != nil {
			errs = append(errs, err)
		} else {
			current = result
		}
	}
	return current, errs
}

// Of creates a pipeline from stages.
func Of[T any](stages ...Stage[T]) *Pipeline[T] {
	return &Pipeline[T]{stages: stages}
}

// Map creates a stage that transforms using a simple function.
func Map[T any](fn func(T) T) Stage[T] {
	return func(input T) (T, error) {
		return fn(input), nil
	}
}

// Validate creates a stage that validates without transforming.
func Validate[T any](fn func(T) error) Stage[T] {
	return func(input T) (T, error) {
		return input, fn(input)
	}
}
