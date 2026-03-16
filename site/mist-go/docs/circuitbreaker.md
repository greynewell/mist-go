---
title: circuitbreaker
description: The circuitbreaker package — Breaker, three-state machine (Closed/Open/HalfOpen), Config, Do, DoWithFallback, and circuit breaker pools.
---

# circuitbreaker

Import path: `github.com/greynewell/mist-go/circuitbreaker`

The `circuitbreaker` package prevents cascading failures when a downstream service becomes unavailable. When consecutive failures exceed a threshold, the breaker opens and rejects requests immediately — instead of waiting for timeouts — giving the downstream service time to recover.

## The three states

```
Closed ──[failures >= Threshold]──► Open
Open ──[Timeout elapsed]──► HalfOpen
HalfOpen ──[probe succeeds]──► Closed
HalfOpen ──[probe fails]──► Open
```

**Closed** — Normal operation. Requests pass through. Each failure increments the consecutive failure counter. When `consecutiveFail >= Config.Threshold`, the breaker opens.

**Open** — The breaker is open. All calls to `Do` return `ErrOpen` immediately without calling the wrapped function. After `Config.Timeout` elapses, the breaker transitions to HalfOpen automatically.

**HalfOpen** — Recovery probing. Up to `Config.HalfOpenMax` concurrent requests are allowed through. A single success closes the circuit. A single failure reopens it.

## Creating a breaker

```go
cb := circuitbreaker.New(circuitbreaker.Config{
    Threshold:   5,                // open after 5 consecutive failures
    Timeout:     30 * time.Second, // try again after 30s
    HalfOpenMax: 1,                // allow 1 probe in half-open state
})
```

Default values (applied when fields are zero):
- `Threshold`: 5
- `Timeout`: 30 seconds
- `HalfOpenMax`: 1

## Wrapping calls

Use `Do` to wrap any function that may fail:

```go
err := cb.Do(ctx, func(ctx context.Context) error {
    return transport.Send(ctx, msg)
})

if errors.Is(err, circuitbreaker.ErrOpen) {
    // Circuit is open. The wrapped function was not called.
    // Return a cached result, degrade gracefully, or propagate the error.
    return serveCachedResponse()
}
if err != nil {
    // The wrapped function returned an error. The failure was counted.
    return err
}
```

Context cancellation errors do not count toward the failure threshold. If the caller gives up (e.g., the request timed out from the caller's perspective), the downstream service did not necessarily fail.

## Fallback

`DoWithFallback` calls the fallback function when the circuit is open, instead of returning `ErrOpen`:

```go
result, err := cb.DoWithFallback(
    ctx,
    func(ctx context.Context) error {
        return callPrimaryModel(ctx)
    },
    func(ctx context.Context, rejectionErr error) error {
        // rejectionErr is ErrOpen — circuit was open.
        log.Warn(ctx, "circuit open, using fallback model")
        return callFallbackModel(ctx)
    },
)
```

## Inspecting state

```go
state := cb.State()
// circuitbreaker.Closed, circuitbreaker.Open, or circuitbreaker.HalfOpen

fmt.Println(state) // "closed", "open", or "half-open"

successes, failures := cb.Counts()
fmt.Printf("successes: %d, failures: %d\n", successes, failures)
```

## Using multiple circuit breakers

For tools that talk to multiple downstream services, create one breaker per service:

```go
type BreakerPool struct {
    infermux   *circuitbreaker.Breaker
    tokentrace *circuitbreaker.Breaker
    schemaflux *circuitbreaker.Breaker
}

func NewBreakerPool() *BreakerPool {
    cfg := circuitbreaker.Config{
        Threshold:   5,
        Timeout:     30 * time.Second,
        HalfOpenMax: 1,
    }
    return &BreakerPool{
        infermux:   circuitbreaker.New(cfg),
        tokentrace: circuitbreaker.New(cfg),
        schemaflux: circuitbreaker.New(cfg),
    }
}

func (p *BreakerPool) SendToInfermux(ctx context.Context, msg *protocol.Message) error {
    return p.infermux.Do(ctx, func(ctx context.Context) error {
        return infermuxTransport.Send(ctx, msg)
    })
}
```

## Combining with retry

Circuit breakers and retry work at different levels. Retry handles transient failures (network hiccups); circuit breakers handle sustained failures (service down). The recommended pattern:

```go
// Retry wraps the individual call; the circuit breaker wraps the retry.
// This means: retry 3 times, and if all 3 fail, that counts as one circuit breaker failure.
err := cb.Do(ctx, func(ctx context.Context) error {
    return retry.Do(ctx, retry.DefaultPolicy, func(ctx context.Context) error {
        return transport.Send(ctx, msg)
    })
})
```

Alternatively, let the circuit breaker operate on individual attempts:

```go
// Each failed attempt (including those that retry internally) counts toward
// the threshold. Faster to open, but may open due to a brief flap.
err := retry.Do(ctx, retry.DefaultPolicy, func(ctx context.Context) error {
    return cb.Do(ctx, func(ctx context.Context) error {
        return transport.Send(ctx, msg)
    })
})
```

Choose based on how quickly you want the circuit to open.

## ErrOpen

```go
var ErrOpen = fmt.Errorf("circuit breaker is open")
```

Check with `errors.Is`:

```go
if errors.Is(err, circuitbreaker.ErrOpen) {
    // circuit is open
}
```
