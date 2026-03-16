---
title: Quick Start
description: Instrument your first inference call with tokentrace in five minutes. Install the package, record a span, see trace output, add a cost alert, and query metrics via HTTP.
---

# Quick Start

This guide walks through the minimum steps to get tokentrace recording traces for a real inference call. By the end you will have a working Go program that records a span, prints it to stdout, fires a cost alert when a threshold is exceeded, and serves aggregated metrics over HTTP.

## 1. Install

```
go get github.com/greynewell/tokentrace
```

Verify the installation:

```
go list -m github.com/greynewell/tokentrace
```

## 2. Instrument an inference call

The core API is three calls: `Start`, `Record`, and `End`.

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/greynewell/tokentrace"
)

func main() {
    // Create a tracer that writes to stdout.
    // In production, swap StdoutTransport for FileTransport or HTTPTransport.
    tracer := tokentrace.New(tokentrace.Config{
        Transport: tokentrace.StdoutTransport(),
    })

    // Start a named trace. The name identifies the workflow or operation.
    trace := tracer.Start("summarize-document")

    // Call your model. Measure latency around the call.
    start := time.Now()
    resp, err := callModel("Summarize the following document in two sentences: ...")
    elapsed := time.Since(start)

    // Record the span. All token and cost fields are first-class.
    trace.Record(tokentrace.Span{
        Model:        "gpt-4o",
        Provider:     "openai",
        PromptTokens: resp.Usage.PromptTokens,
        CompTokens:   resp.Usage.CompletionTokens,
        LatencyMs:    elapsed.Milliseconds(),
        Cost:         tokentrace.Cost("gpt-4o", resp.Usage),
        Status:       tokentrace.StatusOK,
    })
    if err != nil {
        trace.RecordError(err)
    }

    // End the trace. This flushes the span to the transport.
    trace.End()

    fmt.Println("done")
}

// callModel is a placeholder for your actual inference call.
func callModel(prompt string) (*ModelResponse, error) {
    // ... call OpenAI, Anthropic, or any provider
    return &ModelResponse{
        Content: "...",
        Usage: ModelUsage{
            PromptTokens:     512,
            CompletionTokens: 128,
        },
    }, nil
}

type ModelResponse struct {
    Content string
    Usage   ModelUsage
}

type ModelUsage struct {
    PromptTokens     int
    CompletionTokens int
}
```

## 3. See the trace output

Run the program. With `StdoutTransport`, each span is printed as a JSON object:

```json
{
  "trace_id": "7f3a1c2b-e4d5-4f6a-8b9c-0d1e2f3a4b5c",
  "span_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "name": "summarize-document",
  "model": "gpt-4o",
  "provider": "openai",
  "prompt_tokens": 512,
  "comp_tokens": 128,
  "total_tokens": 640,
  "cost_usd": 0.00448,
  "latency_ms": 1240,
  "status": "ok",
  "started_at": "2026-03-15T14:23:01.441Z",
  "ended_at": "2026-03-15T14:23:02.681Z"
}
```

Switch to `FileTransport` to write a JSONL file you can inspect with any JSON tool:

```go
tracer := tokentrace.New(tokentrace.Config{
    Transport: tokentrace.FileTransport("./traces.jsonl"),
})
```

## 4. Add a cost alert

Define an alert rule that fires when total spend in the last hour exceeds $5.00:

```go
tracer := tokentrace.New(tokentrace.Config{
    Transport: tokentrace.StdoutTransport(),
    Alerts: []tokentrace.AlertRule{
        {
            Name:      "hourly-cost-spike",
            Metric:    "total_cost",
            Op:        tokentrace.OpGreaterThan,
            Threshold: 5.00,
            Window:    time.Hour,
            Delivery:  tokentrace.StdoutDelivery(),
        },
    },
})
```

When the rule fires, you will see an alert payload on stdout:

```json
{
  "alert": "hourly-cost-spike",
  "fired_at": "2026-03-15T15:00:00.000Z",
  "metric": "total_cost",
  "value": 5.12,
  "threshold": 5.00,
  "window": "1h"
}
```

To deliver alerts to a webhook instead, replace `StdoutDelivery()` with `HTTPDelivery("https://your-webhook-url")`.

## 5. Query metrics via HTTP

Start the tokentrace HTTP server to expose aggregated metrics:

```go
tracer := tokentrace.New(tokentrace.Config{
    Transport: tokentrace.StdoutTransport(),
    HTTPServer: &tokentrace.HTTPServerConfig{
        Addr: ":9090",
    },
})
// The server runs in the background. Tracer.Start() is non-blocking.
```

Query the metrics endpoint:

```
$ curl http://localhost:9090/metrics/cost?window=1h
{
  "total_usd": 0.42,
  "by_model": {
    "gpt-4o": 0.38,
    "gpt-4o-mini": 0.04
  },
  "window": "1h",
  "span_count": 94
}
```

```
$ curl http://localhost:9090/metrics/latency?window=1h
{
  "p50_ms": 820,
  "p95_ms": 2140,
  "p99_ms": 3810,
  "window": "1h"
}
```

```
$ curl http://localhost:9090/health
{"status": "ok"}
```

## What's next

- [Traces & Spans](/tokentrace/docs/traces/) — All Span fields, nesting spans, custom attributes.
- [Transports](/tokentrace/docs/transports/) — FileTransport, HTTPTransport, MultiTransport configuration.
- [Alerts](/tokentrace/docs/alerts/) — Cost, latency, and quality alert rules with webhook delivery.
- [Metrics Reference](/tokentrace/docs/metrics-reference/) — Every built-in metric name, type, and label.
