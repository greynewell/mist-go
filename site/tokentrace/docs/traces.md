---
title: Traces & Spans
description: The tokentrace trace model — the Span struct, all fields, trace context, nested spans for agents, correlation IDs, custom attributes, and trace IDs for replay.
---

# Traces & Spans

A **trace** represents a complete logical operation in your AI system — a single document summary, a user query answered by an agent, a batch evaluation run. A trace contains one or more **spans**, where each span represents one inference call within that operation. For a simple single-call workflow, a trace contains exactly one span. For a multi-step agent, a trace contains one span per LLM call, linked by parent/child relationships.

## The Span struct

```go
type Span struct {
    // Identity
    TraceID      string            // UUID, set by tracer at Start(); read-only
    SpanID       string            // UUID, generated per span; read-only
    ParentSpanID string            // UUID of parent span, empty for root spans
    Name         string            // operation name, inherited from tracer.Start()
    Caller       string            // optional: calling service or function name

    // Model
    Model        string            // required: model identifier, e.g. "gpt-4o"
    Provider     string            // optional: provider name, e.g. "openai", "anthropic"

    // Tokens — at least one of these must be non-zero
    PromptTokens int               // tokens in the input context
    CompTokens   int               // tokens in the model output (completion)
    TotalTokens  int               // total tokens; computed if zero and both above are set

    // Cost
    Cost         float64           // cost in USD; use tokentrace.Cost() to compute
    CostModel    string            // optional: pricing model used, e.g. "gpt-4o@2025-01"

    // Performance
    LatencyMs    int64             // wall-clock latency, request dispatch to response
    TTFTMs       int64             // time to first token in ms (streaming only)

    // Status
    Status       SpanStatus        // StatusOK, StatusError, or StatusTimeout
    Error        string            // error message if Status != StatusOK

    // Timing
    StartedAt    time.Time         // set automatically by tracer.Start()
    EndedAt      time.Time         // set automatically by trace.End()

    // Custom metadata
    Attributes   map[string]any    // arbitrary key/value pairs
}
```

### Required vs optional fields

**Required:**
- `Model` — tokentrace uses this for cost lookup, model-level metrics, and alert rule filtering.
- At least one token count (`PromptTokens`, `CompTokens`, or `TotalTokens`).

**Strongly recommended:**
- `LatencyMs` — required for latency metrics and latency alert rules. Set to `elapsed.Milliseconds()`.
- `Cost` — required for cost metrics and cost alert rules. Use `tokentrace.Cost()` to compute from token counts.
- `Status` — defaults to `StatusOK` if omitted; set to `StatusError` or `StatusTimeout` when the call fails.

**Optional:**
- `Provider`, `Caller`, `TTFTMs`, `ParentSpanID`, `Attributes`.

### SpanStatus values

```go
const (
    StatusOK      SpanStatus = "ok"
    StatusError   SpanStatus = "error"
    StatusTimeout SpanStatus = "timeout"
)
```

## Computing cost

tokentrace ships a built-in pricing table for common models. Use `tokentrace.Cost()` to compute cost from token counts:

```go
type Usage struct {
    PromptTokens     int
    CompletionTokens int
}

cost := tokentrace.Cost("gpt-4o", Usage{
    PromptTokens:     512,
    CompletionTokens: 128,
})
// cost == 0.00448 (at current gpt-4o pricing)
```

If you use a model not in the built-in table, or if you have a custom rate agreement, provide a `CostRate`:

```go
cost := tokentrace.CostWithRate(
    tokentrace.CostRate{
        PromptPer1M:     2.50, // USD per 1M prompt tokens
        CompletionPer1M: 10.00,
    },
    512, 128,
)
```

## The trace context model

When you call `tracer.Start("operation-name")`, tokentrace:

1. Generates a `TraceID` (UUID v4).
2. Creates a `TraceContext` that holds the trace ID and any propagated context values.
3. Returns a `Trace` handle. All spans recorded via this handle share the same `TraceID`.

The `TraceContext` is not propagated automatically across goroutines or HTTP boundaries — you must pass it explicitly if you want distributed trace correlation. Use `trace.Context()` to retrieve the context and `tracer.FromContext(ctx)` to restore a trace handle from it.

```go
trace := tracer.Start("process-request")
ctx := trace.Context()

// Pass ctx to a function that does its own inference calls
go func() {
    childTrace := tracer.FromContext(ctx)
    childTrace.Record(tokentrace.Span{
        Model:      "gpt-4o-mini",
        // ... this span will carry the same TraceID
    })
    childTrace.End()
}()
```

## Nesting spans for multi-step agents

For agent loops where each step makes one or more inference calls, use child spans to preserve the call hierarchy. Set `ParentSpanID` on child spans:

```go
trace := tracer.Start("agent-run")
rootSpan := tokentrace.Span{
    Model:      "gpt-4o",
    // first step: planning
    PromptTokens: 1024,
    CompTokens:   256,
    LatencyMs:    980,
    Cost:         tokentrace.Cost("gpt-4o", usage),
    Status:       tokentrace.StatusOK,
}
trace.Record(rootSpan)

// Each subsequent step is a child of the root span
for i, step := range agentSteps {
    stepSpan := tokentrace.Span{
        ParentSpanID: rootSpan.SpanID,
        Model:        "gpt-4o",
        Attributes: map[string]any{
            "step":       i,
            "step_name":  step.Name,
        },
        // ... token counts, latency, cost
    }
    trace.Record(stepSpan)
}

trace.End()
```

Nested spans are not required — you can record all spans at the root level and correlate them by `TraceID` if you prefer. The hierarchy is useful when you want to display a tree view in your observability tool or compute per-step costs within a single trace.

## Correlation IDs

If your system has an existing request ID or session ID that you want to link to trace data, add it as an attribute:

```go
trace.Record(tokentrace.Span{
    Model: "gpt-4o",
    Attributes: map[string]any{
        "request_id": requestID,
        "session_id": sessionID,
        "user_id":    userID,
    },
    // ...
})
```

Attributes are indexed and queryable via the HTTP API. `GET /traces?attr.request_id=abc123` returns all traces containing a span with that attribute value.

## Custom attributes

Attributes accept string, int, float64, and bool values. Keys must be non-empty strings. A few reserved prefixes have special behavior:

- `eval.*` — attributes with this prefix are treated as quality scores. `eval.score` is used by the `quality_drop` alert rule. Values must be float64 in [0, 1].
- `budget.*` — reserved for the `Budget` helper.
- `tt.*` — reserved for tokentrace internal metadata.

Example — attaching a matchspec eval score:

```go
trace.Record(tokentrace.Span{
    Model: "gpt-4o",
    Attributes: map[string]any{
        "eval.score":    0.87,
        "eval.suite":    "summarization-v2",
        "workflow":      "document-summary",
        "document_type": "legal",
    },
    // ...
})
```

## Trace IDs and replay

The `TraceID` returned by `tracer.Start()` can be stored and used later to retrieve all spans in a trace from the HTTP API:

```go
trace := tracer.Start("summarize-document")
traceID := trace.ID() // store this alongside the response

// Later, retrieve the full trace
// GET /traces/{traceID}
```

This makes it possible to replay a trace for debugging — you can see exactly what tokens were consumed, at what cost, and with what latency for any specific user request. Traces are retained for the duration configured in `tokentrace.yml` (default: 7 days when using FileTransport or the HTTP server with disk-backed storage).

## Next steps

- [Metrics](/tokentrace/docs/metrics/) — How spans are aggregated into queryable metrics.
- [Alerts](/tokentrace/docs/alerts/) — Alert rules that reference span attributes and metrics.
- [Agent Instrumentation](/tokentrace/docs/agent-instrumentation/) — Guide to instrumenting a full agent loop.
