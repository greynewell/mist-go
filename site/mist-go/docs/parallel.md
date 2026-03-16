---
title: parallel
description: The parallel package — Pool, Map, Do, FanOut, bounded concurrency, context cancellation, and collecting results in input order.
---

# parallel

Import path: `github.com/greynewell/mist-go/parallel`

The `parallel` package provides a worker pool for bounded concurrent execution. The three functions — `Map`, `Do`, and `FanOut` — use Go generics and work over any input and output types. Results from `Map` are always returned in input order, regardless of which goroutine finished first.

## Creating a pool

```go
pool := parallel.NewPool(8) // up to 8 concurrent goroutines
```

The pool is a concurrency bound, not a goroutine pool. Goroutines are spawned per job; the semaphore limits how many run simultaneously. This is simpler and more correct than maintaining a persistent goroutine pool, especially with context cancellation.

A pool with fewer than 1 worker is automatically set to 1.

## Map

`Map` applies a function to each element of a slice concurrently and returns results in the same order as the inputs:

```go
type Result[T any] struct {
    Value T
    Err   error
}

results := parallel.Map(ctx, pool, examples, func(ctx context.Context, ex Example) (Score, error) {
    return scoreExample(ctx, ex)
})

for i, r := range results {
    if r.Err != nil {
        fmt.Printf("example %d failed: %v\n", i, r.Err)
        continue
    }
    fmt.Printf("example %d: %.2f\n", i, r.Value.Score)
}
```

`Map` is generic over both input and output types. The function signature is `func(context.Context, In) (Out, error)`.

**Order preservation** — Results are written to a pre-allocated `[]Result[Out]` at the original index of each input. There is no need to sort or correlate results after the call.

**Context cancellation** — If the context is cancelled before all inputs are launched, remaining inputs get `Result{Err: ctx.Err()}` without executing the function. In-flight goroutines continue until they finish (they receive the cancelled context, but are not forcibly stopped). This is the safe default: goroutines that are already running should complete their work rather than leave partial state.

## Do

`Do` runs a function for each input concurrently and returns the first error encountered, or nil if all succeed:

```go
err := parallel.Do(ctx, pool, tasks, func(ctx context.Context, task Task) error {
    return executeTask(ctx, task)
})
if err != nil {
    return fmt.Errorf("one or more tasks failed: %w", err)
}
```

`Do` is implemented as `Map` with a `struct{}` output type. If you need to know which tasks failed (not just whether any did), use `Map` directly.

## FanOut

`FanOut` sends a single input to multiple functions concurrently and collects all results:

```go
prompt := "Explain the difference between mutex and channel."

scorers := []func(context.Context, string) (Score, error){
    exactMatchScorer,
    semanticSimilarityScorer,
    llmJudgeScorer,
}

results := parallel.FanOut(ctx, pool, prompt, scorers)

for i, r := range results {
    if r.Err != nil {
        fmt.Printf("scorer %d failed: %v\n", i, r.Err)
        continue
    }
    fmt.Printf("scorer %d: %.2f\n", i, r.Value.Score)
}
```

`FanOut` is implemented as `Map` over a slice of functions, passing the same input to each.

## Running eval harnesses in parallel

A typical matchspec use of `Map` to run multiple harnesses concurrently:

```go
type HarnessResult struct {
    Name     string
    Score    float64
    Passed   bool
}

pool := parallel.NewPool(cfg.Workers)
results := parallel.Map(ctx, pool, harnesses,
    func(ctx context.Context, h Harness) (HarnessResult, error) {
        score, err := h.Run(ctx)
        if err != nil {
            return HarnessResult{Name: h.Name}, err
        }
        return HarnessResult{
            Name:   h.Name,
            Score:  score,
            Passed: score >= h.Threshold,
        }, nil
    },
)

allPassed := true
for _, r := range results {
    if r.Err != nil {
        fmt.Printf("FAIL  %s: error: %v\n", r.Value.Name, r.Err)
        allPassed = false
        continue
    }
    status := "PASS"
    if !r.Value.Passed {
        status = "FAIL"
        allPassed = false
    }
    fmt.Printf("%s  %s: %.2f\n", status, r.Value.Name, r.Value.Score)
}
```

## Collecting all errors

`Map` preserves all errors. If you want to collect all failures rather than just the first:

```go
results := parallel.Map(ctx, pool, tasks, func(ctx context.Context, t Task) (any, error) {
    return nil, executeTask(ctx, t)
})

var errs []error
for i, r := range results {
    if r.Err != nil {
        errs = append(errs, fmt.Errorf("task %d: %w", i, r.Err))
    }
}
if len(errs) > 0 {
    return fmt.Errorf("%d tasks failed: %v", len(errs), errors.Join(errs...))
}
```

## Choosing pool size

Use `runtime.NumCPU()` as a starting point for CPU-bound work. For I/O-bound work (inference calls, network requests), you can use a higher multiplier:

```go
// CPU-bound (scoring, parsing, computation)
pool := parallel.NewPool(runtime.NumCPU())

// I/O-bound (inference calls, HTTP requests)
pool := parallel.NewPool(runtime.NumCPU() * 4)

// Respect a configured limit
pool := parallel.NewPool(cfg.Workers)
```

For inference calls specifically, the pool size should also respect the rate limits and connection limits of the downstream model provider.
