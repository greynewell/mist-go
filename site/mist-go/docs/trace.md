---
title: trace
description: The trace package — Span, Start, SetAttr, W3C Trace Context propagation, TraceID/SpanID format, and integration with tokentrace.
---

# trace

Import path: `github.com/greynewell/mist-go/trace`

The `trace` package provides context-based distributed tracing for MIST tools. Spans are created, propagated through `context.Context`, and converted to `protocol.TraceSpan` for transport across tool boundaries. The design integrates with tokentrace for token-level observability while remaining usable as a standalone tracing system.

## Span

`Span` represents a single unit of work within a trace:

```go
type Span struct {
    TraceID   string // shared across all spans in a request
    SpanID    string // unique ID for this span
    ParentID  string // SpanID of the parent span, or empty for root
    Operation string // human-readable name for this work unit
    StartNS   int64  // Unix nanoseconds at span creation
    Status    string // "ok" or "error", set by End
    EndNS     int64  // Unix nanoseconds at End, 0 if still open

    // attrs is private; access via SetAttr and Attrs
}
```

TraceID and SpanID are 32-character lowercase hex strings (128 bits from `crypto/rand`). They satisfy the W3C Trace Context format requirements.

## Starting spans

`Start` creates a new span and attaches it to a new context. If the context already contains a span, the new span inherits its trace ID and uses the parent's span ID as its `ParentID`:

```go
ctx, span := trace.Start(ctx, "inference")
defer span.End("ok")

// Do work...
result, err := callModel(ctx, req)
if err != nil {
    span.End("error")
    return err
}
```

The `defer span.End("ok")` pattern is idiomatic, but you may call `End` explicitly when you need to pass the status based on whether an error occurred.

For root spans (the first span in a request), the trace ID is generated randomly. For child spans, `Start` reads the parent span from the context and inherits its trace ID:

```go
// Root span.
ctx, rootSpan := trace.Start(ctx, "eval-run")
defer rootSpan.End("ok")

// Child span — inherits rootSpan.TraceID, sets ParentID = rootSpan.SpanID.
ctx, childSpan := trace.Start(ctx, "infer-request")
defer childSpan.End("ok")

fmt.Println(rootSpan.TraceID == childSpan.TraceID) // true
fmt.Println(childSpan.ParentID == rootSpan.SpanID)  // true
```

## Starting spans with an explicit trace ID

When receiving a message from another tool that includes a trace ID, use `StartWithTraceID` to continue the existing trace:

```go
func handleMessage(ctx context.Context, msg *protocol.Message) error {
    var req protocol.InferRequest
    msg.Decode(&req)

    traceID := req.Meta["trace_id"]
    ctx, span := trace.StartWithTraceID(ctx, traceID, "handle-infer-request")
    defer span.End("ok")

    // span.TraceID == traceID (unless traceID was invalid, in which case a new one is generated)
    return doWork(ctx)
}
```

`StartWithTraceID` validates the trace ID with `ValidID` before using it. Invalid IDs (empty, over 256 characters, or containing non-ASCII printable characters) are replaced with a new random ID to prevent log injection.

## Setting attributes

`SetAttr` attaches key-value pairs to a span. Attributes are safe to set from multiple goroutines:

```go
span.SetAttr("model", "claude-sonnet-4-5-20250929")
span.SetAttr("provider", "anthropic")
span.SetAttr("tokens_in", int64(128))
span.SetAttr("tokens_out", int64(512))
span.SetAttr("cost_usd", 0.00192)
span.SetAttr("error", err.Error()) // only on failure
```

Common attribute names used across MIST tools:

| Key | Type | Description |
|-----|------|-------------|
| `model` | string | Model name |
| `provider` | string | Provider name |
| `tokens_in` | int64 | Input token count |
| `tokens_out` | int64 | Output token count |
| `cost_usd` | float64 | Estimated cost in USD |
| `latency_ms` | float64 | Request latency |
| `finish_reason` | string | Model finish reason |
| `error` | string | Error message on failure |
| `suite` | string | Eval suite name |
| `task` | string | Eval task name |

`Attrs` returns a copy of all attributes:

```go
attrs := span.Attrs() // map[string]any
```

## Reading span timing

```go
// Duration in nanoseconds (0 if span is still open).
ns := span.DurationNS()

// Duration in milliseconds.
ms := span.DurationMS()
```

## Extracting spans from context

```go
// Get the current span, or nil if none.
span := trace.FromContext(ctx)

// Get the trace ID directly (empty string if no span).
traceID := trace.TraceID(ctx)

// Get the span ID directly (empty string if no span).
spanID := trace.SpanID(ctx)
```

## W3C Trace Context

The `trace` package implements W3C Trace Context for propagating trace state across HTTP boundaries.

**Injecting trace context into outgoing requests:**

```go
req, _ := http.NewRequestWithContext(ctx, "POST", targetURL, body)
trace.InjectHTTP(ctx, req.Header)
// Sets: Traceparent: 00-{32hex}-{16hex}-01
//       Tracestate: mist={32hex-span-id}
```

**Extracting trace context from incoming requests:**

```go
func handler(w http.ResponseWriter, r *http.Request) {
    ctx, span := trace.ExtractHTTP(r.Context(), r.Header, "handle-request")
    defer span.End("ok")
    // span.TraceID is from the incoming Traceparent header, or new if absent
}
```

The `traceparent` header format is `00-{trace_id_32hex}-{parent_id_16hex}-01`. MIST generates 32-hex span IDs; for W3C compatibility, the last 16 hex characters of the span ID are used as the `parent-id` field.

**Parsing and formatting traceparent headers directly:**

```go
traceID, parentID, ok := trace.ParseTraceparent("00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
// traceID = "4bf92f3577b34da6a3ce929d0e0e4736"
// parentID = "00f067aa0ba902b7"

header := trace.FormatTraceparent(traceID, parentID)
// "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
```

## Sending spans to tokentrace

Convert a completed span to `protocol.TraceSpan` and send it to tokentrace via any transport:

```go
ctx, span := trace.Start(ctx, "inference")
result, err := callModel(ctx, req)
if err != nil {
    span.SetAttr("error", err.Error())
    span.End("error")
} else {
    span.SetAttr("tokens_out", result.TokensOut)
    span.SetAttr("cost_usd", result.CostUSD)
    span.End("ok")
}

// Ship to tokentrace.
payload := protocol.TraceSpan{
    TraceID:   span.TraceID,
    SpanID:    span.SpanID,
    ParentID:  span.ParentID,
    Operation: span.Operation,
    StartNS:   span.StartNS,
    EndNS:     span.EndNS,
    Status:    span.Status,
    Attrs:     span.Attrs(),
}
msg, _ := protocol.New(protocol.SourceMatchSpec, protocol.TypeTraceSpan, payload)
tokentraceTr.Send(ctx, msg)
```

## Generating IDs

To generate a standalone trace or span ID (128-bit random hex):

```go
id := trace.NewID() // "a3f2b1c4d5e6f7a8b9c0d1e2f3a4b5c6"
```

`ValidID` checks whether a string is safe to use as a trace or span ID:

```go
trace.ValidID("a3f2b1c4")       // true
trace.ValidID("")               // false
trace.ValidID("\x00malicious") // false (non-printable characters)
```

The check is a log injection defense: IDs that come from external sources are validated before being included in log entries or span attributes.
