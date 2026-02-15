// Package parallel provides concurrency primitives for MIST tools:
// worker pools, rate limiters, and fan-out patterns.
package parallel

import (
	"context"
	"sync"
)

// Pool executes work functions concurrently with a bounded number of
// goroutines. Use Map for transforming slices and Do for side-effecting work.
type Pool struct {
	workers int
}

// NewPool creates a pool with the given concurrency limit.
func NewPool(workers int) *Pool {
	if workers < 1 {
		workers = 1
	}
	return &Pool{workers: workers}
}

// Result holds the output and error from a single work item.
type Result[T any] struct {
	Value T
	Err   error
}

// Map applies fn to each input concurrently, returning results in input order.
// It stops launching new work if ctx is cancelled but waits for in-flight
// goroutines to finish.
func Map[In, Out any](ctx context.Context, p *Pool, inputs []In, fn func(context.Context, In) (Out, error)) []Result[Out] {
	results := make([]Result[Out], len(inputs))
	sem := make(chan struct{}, p.workers)
	var wg sync.WaitGroup

	for i, input := range inputs {
		if ctx.Err() != nil {
			results[i] = Result[Out]{Err: ctx.Err()}
			continue
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(idx int, in In) {
			defer wg.Done()
			defer func() { <-sem }()

			val, err := fn(ctx, in)
			results[idx] = Result[Out]{Value: val, Err: err}
		}(i, input)
	}

	wg.Wait()
	return results
}

// Do runs fn for each input concurrently and collects errors.
// It returns the first error encountered, or nil if all succeed.
func Do[In any](ctx context.Context, p *Pool, inputs []In, fn func(context.Context, In) error) error {
	results := Map(ctx, p, inputs, func(ctx context.Context, in In) (struct{}, error) {
		return struct{}{}, fn(ctx, in)
	})
	for _, r := range results {
		if r.Err != nil {
			return r.Err
		}
	}
	return nil
}

// FanOut sends the same input to multiple functions concurrently and
// collects all results.
func FanOut[In, Out any](ctx context.Context, p *Pool, input In, fns []func(context.Context, In) (Out, error)) []Result[Out] {
	return Map(ctx, p, fns, func(ctx context.Context, fn func(context.Context, In) (Out, error)) (Out, error) {
		return fn(ctx, input)
	})
}
