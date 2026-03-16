---
title: metrics
description: The metrics package — Counter, Gauge, Histogram, Registry, lock-free atomic operations, JSON export, and HTTP handler.
---

# metrics

Import path: `github.com/greynewell/mist-go/metrics`

The `metrics` package provides lightweight, zero-dependency counters, gauges, and histograms for MIST tools. All types use lock-free atomic operations for high-throughput recording. A `Registry` groups metrics together, supports label pairs, and exposes a JSON HTTP handler.

## Registry

All metrics are created through a `Registry`:

```go
reg := metrics.NewRegistry()
```

The registry deduplicates: calling `reg.Counter("requests_total")` twice with the same name and labels returns the same counter. All methods are safe for concurrent use.

## Counter

A counter is a monotonically increasing integer. Use it for events: requests received, errors encountered, bytes sent.

```go
// Create a counter (no labels).
requests := reg.Counter("http_requests_total")
requests.Inc()       // +1
requests.Add(10)     // +10
fmt.Println(requests.Value()) // current count
```

**With labels:**

```go
// Label pairs are key, value, key, value, ...
getReqs := reg.Counter("http_requests_total", "method", "GET")
postReqs := reg.Counter("http_requests_total", "method", "POST")

getReqs.Inc()
postReqs.Inc()
```

Labels are appended to the metric name in the registry key as `name{key,value,key,value}`. Two calls with the same name and labels return the same `*Counter`.

## Gauge

A gauge is a float64 that can go up and down. Use it for current state: active connections, queue depth, memory usage.

```go
connections := reg.Gauge("active_connections")
connections.Set(42.0)
connections.Inc()        // +1.0
connections.Dec()        // -1.0
connections.Add(-5.0)    // -5.0
fmt.Println(connections.Value())
```

Gauge operations use CAS loops for atomic float64 updates without locks.

## Histogram

A histogram tracks the distribution of observed values using configurable bucket boundaries. Use it for latencies, sizes, and anything where percentiles matter.

```go
latency := reg.Histogram(
    "request_duration_ms",
    metrics.DefaultBuckets, // [1, 5, 10, 25, 50, 100, 250, 500, 1000, 5000, 10000]
)

latency.Observe(42.5)
latency.Observe(250.0)
latency.Observe(1200.0)
```

`DefaultBuckets` are the default boundaries for latency in milliseconds. You can supply custom buckets for other units:

```go
// Token count histogram.
tokenHist := reg.Histogram(
    "tokens_out",
    []float64{100, 500, 1000, 2000, 4000, 8000, 16000},
    "model", "claude-sonnet-4-5",
)
```

Buckets are automatically sorted. Values are accumulated using lock-free atomic operations.

## Histogram snapshots

Call `Snapshot()` to get a point-in-time view of histogram state:

```go
snap := latency.Snapshot()

fmt.Printf("count: %d\n", snap.Count)
fmt.Printf("sum: %.2f ms\n", snap.Sum)
fmt.Printf("min: %.2f ms\n", snap.Min)
fmt.Printf("max: %.2f ms\n", snap.Max)
fmt.Printf("avg: %.2f ms\n", snap.Avg())
fmt.Printf("p50: %.2f ms\n", snap.Percentile(50))
fmt.Printf("p95: %.2f ms\n", snap.Percentile(95))
fmt.Printf("p99: %.2f ms\n", snap.Percentile(99))
```

`Percentile` uses linear interpolation within the bucket that contains the target rank. It is an estimate, not exact — the accuracy depends on bucket granularity.

Histogram buckets are stored as raw (non-cumulative) counts internally and converted to cumulative at snapshot time. This means `Snapshot()` always returns a consistent view: the cumulative count at each boundary equals the number of observations at or below that boundary.

## Registry snapshot

`Registry.Snapshot()` returns a point-in-time view of all registered metrics:

```go
type RegistrySnapshot struct {
    Counters   map[string]CounterSnapshot
    Gauges     map[string]GaugeSnapshot
    Histograms map[string]HistogramSnapshot
}

snap := reg.Snapshot()
for name, c := range snap.Counters {
    fmt.Printf("%s = %d\n", name, c.Value)
}
```

The snapshot keys are the registry keys (`name` or `name{label,value,...}`), not the metric names.

## HTTP handler

`Registry.Handler()` returns an `http.HandlerFunc` that serves the current snapshot as JSON:

```go
mux := http.NewServeMux()
mux.Handle("GET /metricsz", reg.Handler())
```

The response is a JSON object with `counters`, `gauges`, and `histograms` keys. Example response:

```json
{
  "counters": {
    "http_requests_total{method,GET}": {
      "name": "http_requests_total",
      "labels": ["method", "GET"],
      "value": 1024
    }
  },
  "gauges": {
    "active_connections": {
      "name": "active_connections",
      "value": 7.0
    }
  },
  "histograms": {
    "request_duration_ms": {
      "name": "request_duration_ms",
      "count": 500,
      "sum": 12450.5,
      "min": 1.2,
      "max": 4800.0,
      "buckets": {
        "1": 0,
        "5": 12,
        "10": 45,
        "25": 120,
        "50": 280,
        "100": 390,
        "250": 460,
        "500": 498,
        "1000": 499,
        "5000": 500,
        "10000": 500
      }
    }
  }
}
```

Histogram bucket keys in JSON are string-formatted float values (e.g., `"50"`, `"1000"`). This is because JSON does not support float64 map keys natively.

## Full example: instrumenting an inference call

```go
package main

import (
    "context"
    "net/http"
    "time"

    "github.com/greynewell/mist-go/metrics"
)

var reg = metrics.NewRegistry()
var (
    inferRequests = reg.Counter("infer_requests_total")
    inferErrors   = reg.Counter("infer_errors_total")
    inferLatency  = reg.Histogram("infer_duration_ms", metrics.DefaultBuckets)
    activeInfer   = reg.Gauge("infer_active")
)

func callModel(ctx context.Context, prompt string) (string, error) {
    inferRequests.Inc()
    activeInfer.Inc()
    defer activeInfer.Dec()

    start := time.Now()
    result, err := doInference(ctx, prompt)
    inferLatency.Observe(float64(time.Since(start).Milliseconds()))

    if err != nil {
        inferErrors.Inc()
        return "", err
    }
    return result, nil
}

func main() {
    http.HandleFunc("GET /metricsz", reg.Handler())
    http.ListenAndServe(":8080", nil)
}
```
