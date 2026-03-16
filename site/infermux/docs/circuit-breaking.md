---
title: "Circuit Breaking"
description: "Automatic failover and recovery. Circuit breaker state machine, thresholds, half-open probes, and manual control."
---

# Circuit Breaking

infermux maintains an independent circuit breaker for each provider. The circuit breaker protects against cascading failures: when a provider starts returning errors or timing out, the circuit opens and requests stop being sent to it. This happens automatically, without intervention. When the provider recovers, the circuit closes and traffic resumes.

Circuit breaking is built on `mist-go/circuitbreaker`. Every configuration option here maps directly to `circuitbreaker.Config` fields.

## The state machine

A circuit breaker has three states:

**Closed** — normal operation. All requests are forwarded to the provider. The circuit breaker monitors error rate and latency. If either crosses the configured threshold, the circuit transitions to open.

**Open** — the provider is excluded from routing. Requests are not forwarded. The circuit stays open for the configured `recovery_window` duration, then transitions to half-open.

**Half-open** — recovery testing. The circuit allows exactly one request through (a probe). If the probe succeeds, the circuit transitions back to closed. If the probe fails or times out, the circuit returns to open and the recovery window resets.

```
                ┌──────────────────────────────────────────────┐
                │                                              │
         threshold crossed                              probe succeeds
                │                                              │
     CLOSED ────┤                                    HALF-OPEN │
                └────► OPEN ───► (recovery_window) ────────────┘
                              ◄──── probe fails ───────────────┘
```

## Configuration

```yaml
circuit_breaker:
  error_rate_threshold: 0.5      # open when error rate exceeds 50%
  consecutive_failures: 5        # open after N consecutive failures regardless of rate
  latency_p95_ms: 5000           # open when p95 latency exceeds this (ms)
  window_seconds: 60             # sliding window for error rate calculation
  min_requests: 10               # minimum requests before error rate is evaluated
  recovery_window: 30s           # how long to stay open before trying half-open
  probe_timeout: 5s              # timeout for the half-open probe request
```

These are global defaults applied to every provider. You can override them per provider:

```yaml
providers:
  - name: openai
    type: openai
    api_key: "${OPENAI_API_KEY}"
    circuit_breaker:
      error_rate_threshold: 0.3  # more sensitive: open at 30% errors
      consecutive_failures: 3
      recovery_window: 15s

  - name: anthropic
    type: anthropic
    api_key: "${ANTHROPIC_API_KEY}"
    circuit_breaker:
      error_rate_threshold: 0.7  # more tolerant: only open at 70% errors
      consecutive_failures: 10
      recovery_window: 60s
```

## Threshold semantics

**error_rate_threshold** is evaluated over the `window_seconds` sliding window, but only when the provider has served at least `min_requests` during that window. This prevents a single failure from opening the circuit on a lightly-loaded provider.

An error is any response with HTTP status 5xx, any timeout (including the provider returning a response after `timeout` is exceeded), and any request that fails to connect. 429 (rate limit) responses are not counted as circuit-breaking errors by default — they are handled separately by the rate limit tracking layer. To count 429s as errors:

```yaml
circuit_breaker:
  treat_rate_limit_as_error: true
```

**consecutive_failures** provides a fast-open path. If a provider returns N errors in a row without any success, the circuit opens immediately, regardless of the error rate over the window. This handles the case where a provider has just gone completely down.

**latency_p95_ms** triggers circuit opening when the 95th-percentile latency over the window exceeds the threshold. This catches slow providers that aren't returning errors but are contributing to user-facing latency. The p95 is computed over the same sliding window as the error rate.

## Half-open recovery probes

When the recovery window expires, the circuit enters half-open state and sends one probe request to the provider. The probe is a real request from the queue — the next request that would have gone to this provider if it were closed. There is no synthetic probe; infermux does not send a separate health-check request during half-open state.

If the probe request succeeds (2xx response within `probe_timeout`), the circuit closes and the provider is immediately eligible for all subsequent requests. If the probe fails or times out, the circuit opens again and the recovery window resets from zero.

The recovery window uses exponential backoff when configured:

```yaml
circuit_breaker:
  recovery_window: 30s
  recovery_backoff_multiplier: 2.0   # double the window on each consecutive failure
  recovery_backoff_max: 600s         # cap at 10 minutes
```

With these settings, a provider that fails its first probe waits 60 seconds before the next probe, then 120, then 240, up to 600 seconds.

## Manual circuit control

The management API exposes endpoints to manually open, close, or reset a circuit:

```bash
# Open a circuit manually (force provider offline)
curl -X POST http://localhost:8080/_infermux/providers/openai/circuit/open

# Close a circuit manually (force provider online, bypassing recovery probe)
curl -X POST http://localhost:8080/_infermux/providers/openai/circuit/close

# Reset a circuit to closed with cleared statistics
curl -X POST http://localhost:8080/_infermux/providers/openai/circuit/reset
```

Manual open is useful when you know a provider is degraded and want to preemptively remove it from routing before the circuit opens automatically. Manual close is useful during maintenance when you want to restore a provider to service immediately after resolving an incident.

## Circuit state in the status API

```bash
curl http://localhost:8080/_infermux/providers
```

```json
[
  {
    "name": "openai",
    "type": "openai",
    "healthy": true,
    "circuit": "closed",
    "error_rate": 0.02,
    "p95_latency_ms": 412,
    "requests_last_minute": 847
  },
  {
    "name": "anthropic",
    "type": "anthropic",
    "healthy": true,
    "circuit": "open",
    "circuit_opened_at": "2026-03-15T14:22:01Z",
    "circuit_recovery_at": "2026-03-15T14:22:31Z",
    "error_rate": 0.68,
    "p95_latency_ms": 8200,
    "requests_last_minute": 103
  }
]
```

## Integration with mist-go/circuitbreaker

infermux creates one `circuitbreaker.Breaker` per provider:

```go
import "github.com/greynewell/mist-go/circuitbreaker"

cfg := circuitbreaker.Config{
    ErrorRateThreshold:  0.5,
    ConsecutiveFailures: 5,
    WindowSeconds:       60,
    MinRequests:         10,
    RecoveryWindow:      30 * time.Second,
    ProbeTimeout:        5 * time.Second,
}

breaker := circuitbreaker.New(cfg)
```

When using infermux as a library, you can access the breaker for each provider directly from the router and subscribe to state change events:

```go
router.OnCircuitStateChange(func(provider string, from, to circuitbreaker.State) {
    log.Printf("provider %s: circuit %s -> %s", provider, from, to)
    // emit to your alerting system
})
```

State change events fire synchronously before the first request is affected by the new state.
