---
title: Parallel Execution
slug: parallel
order: 4
---

# Parallel Execution

The `parallel` package provides concurrency primitives with zero
external dependencies.

## Worker Pool

Process items concurrently with a bounded number of goroutines:

```go
pool := parallel.NewPool(8)

results := parallel.Map(ctx, pool, urls, func(ctx context.Context, url string) (Response, error) {
    return fetch(ctx, url)
})

for _, r := range results {
    if r.Err != nil {
        log.Printf("error: %v", r.Err)
        continue
    }
    process(r.Value)
}
```

Results are always returned in input order regardless of completion order.

## Fan-Out

Send the same input to multiple functions:

```go
results := parallel.FanOut(ctx, pool, prompt, []func(ctx, string) (string, error){
    callAnthropic,
    callOpenAI,
    callGoogle,
})
```

## Rate Limiter

Enforce a maximum number of operations per time window:

```go
limiter := parallel.NewRateLimiter(100, time.Second) // 100 ops/sec

for _, item := range items {
    if err := limiter.Wait(ctx); err != nil {
        break
    }
    process(item)
}
```
