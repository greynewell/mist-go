---
title: Instrument Your First Inference Call
description: Add tokentrace to an existing Go program, instrument a single inference call with Start, Record, and End, and inspect the structured trace output.
---

# Instrument Your First Inference Call

<div class="tutorial-meta">
  <span class="meta-tag">beginner</span>
  <span class="meta-tag">10 min</span>
  <span class="meta-tag">traces</span>
  <span class="meta-tag">go api</span>
</div>

In this tutorial you will add tokentrace to a Go program that calls a language model and see exactly what a structured trace looks like. By the end, you will have a working example you can adapt to any inference call in your own code.

**What you will build:** A Go program that calls a language model to summarize a document, records the inference call as a structured trace, and prints the trace to stdout as JSON.

**Prerequisites:**
- Go 1.21 or later
- A Go module (`go.mod` in your project directory)
- Basic familiarity with Go

---

<div class="step">
<div class="step-number">Step 1</div>

## Install tokentrace

Add the package to your module:

```
go get github.com/greynewell/tokentrace
```

Verify:

```
go list -m github.com/greynewell/tokentrace
```

You should see `github.com/greynewell/tokentrace v0.x.y`.
</div>

<div class="step">
<div class="step-number">Step 2</div>

## Create the program

Create `main.go` in your project directory:

```go
package main

import (
    "context"
    "fmt"
    "os"
    "time"

    "github.com/greynewell/tokentrace"
)

func main() {
    // Create a tracer that writes to stdout.
    // StdoutTransport is the simplest option for development —
    // every span is printed as JSON when trace.End() is called.
    tracer := tokentrace.New(tokentrace.Config{
        Transport: tokentrace.StdoutTransport(),
    })

    // Instrument an inference call.
    if err := summarizeDocument(tracer, "The quick brown fox..."); err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }
}

func summarizeDocument(tracer *tokentrace.Tracer, document string) error {
    ctx := context.Background()

    // Start a trace. The string argument is the operation name.
    // It appears in the "name" field of every span this trace records.
    trace := tracer.Start("summarize-document")

    // Measure the wall-clock time around the model call.
    start := time.Now()
    resp, err := callModel(ctx, "Summarize this document in two sentences: "+document)
    elapsed := time.Since(start)

    if err != nil {
        // RecordError records a span with status "error" automatically.
        trace.RecordError(err)
        trace.End()
        return fmt.Errorf("model call failed: %w", err)
    }

    // Record the span with all the fields we care about.
    trace.Record(tokentrace.Span{
        Model:        "gpt-4o",
        Provider:     "openai",
        PromptTokens: resp.Usage.PromptTokens,
        CompTokens:   resp.Usage.CompletionTokens,
        LatencyMs:    elapsed.Milliseconds(),
        Cost:         tokentrace.Cost("gpt-4o", resp.Usage),
        Status:       tokentrace.StatusOK,
    })

    // End the trace. This flushes the span to the transport.
    trace.End()

    fmt.Println("Summary:", resp.Content)
    return nil
}
```

</div>

<div class="step">
<div class="step-number">Step 3</div>

## Add a stub model function

For this tutorial, we use a stub instead of a real API call so you can run the example without an API key. Add this to `main.go`:

```go
// ModelResponse represents a response from a language model API.
type ModelResponse struct {
    Content string
    Usage   ModelUsage
}

type ModelUsage struct {
    PromptTokens     int
    CompletionTokens int
}

// callModel is a stub. In real code, replace this with your actual
// API call to OpenAI, Anthropic, or any other provider.
func callModel(ctx context.Context, prompt string) (*ModelResponse, error) {
    // Simulate ~600 ms latency and realistic token counts.
    time.Sleep(600 * time.Millisecond)
    return &ModelResponse{
        Content: "The document describes a fox. It is notable for its speed and color.",
        Usage: ModelUsage{
            PromptTokens:     len(prompt) / 4, // rough approximation
            CompletionTokens: 18,
        },
    }, nil
}
```

</div>

<div class="step">
<div class="step-number">Step 4</div>

## Run the program

```
go run main.go
```

You will see two lines of output. The first is the summary from the stub model. The second is the trace span as JSON, written to stdout by `StdoutTransport`:

```json
{
  "trace_id": "7f3a1c2b-e4d5-4f6a-8b9c-0d1e2f3a4b5c",
  "span_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "name": "summarize-document",
  "model": "gpt-4o",
  "provider": "openai",
  "prompt_tokens": 14,
  "comp_tokens": 18,
  "total_tokens": 32,
  "cost_usd": 0.000224,
  "latency_ms": 601,
  "status": "ok",
  "started_at": "2026-03-15T14:23:01.441Z",
  "ended_at": "2026-03-15T14:23:02.042Z"
}
```

Every field is structured and queryable. `trace_id` links all spans from the same logical operation. `cost_usd` is computed automatically from the token counts and model name using tokentrace's built-in pricing table. `latency_ms` is the elapsed time you measured.

</div>

<div class="step">
<div class="step-number">Step 5</div>

## Switch to FileTransport

Writing to stdout is fine for development, but you usually want to keep traces in a file. Change the transport:

```go
tracer := tokentrace.New(tokentrace.Config{
    Transport: tokentrace.FileTransport("./traces.jsonl"),
})
```

Run the program again. Now the span is appended to `traces.jsonl` as a line of JSON. You can inspect it with `jq`:

```
cat traces.jsonl | jq .
```

Each run appends a new line. After a few runs:

```
cat traces.jsonl | jq '.cost_usd' | paste -sd+ | bc
```

This gives you the total cost across all recorded calls — a one-liner demonstration of what cost visibility looks like before you have a dashboard.

</div>

<div class="step">
<div class="step-number">Step 6</div>

## Add a custom attribute

Attributes let you attach any metadata to a span. Add a `caller` field and a `document_type` attribute so you can group traces by document category later:

```go
trace.Record(tokentrace.Span{
    Model:        "gpt-4o",
    Provider:     "openai",
    PromptTokens: resp.Usage.PromptTokens,
    CompTokens:   resp.Usage.CompletionTokens,
    LatencyMs:    elapsed.Milliseconds(),
    Cost:         tokentrace.Cost("gpt-4o", resp.Usage),
    Status:       tokentrace.StatusOK,
    Caller:       "summarizer-service",
    Attributes: map[string]any{
        "document_type": "article",
        "user_id":       "user_42",
    },
})
```

The `Caller` field appears as a top-level field in the span JSON and is available as a grouping dimension in all built-in metrics. The `Attributes` map appears under `"attributes"` in the JSON and is queryable via the HTTP API.

</div>

<div class="step">
<div class="step-number">Step 7 — Next steps</div>

## Where to go from here

You have a working instrumented inference call. The span you recorded contains everything needed for cost accounting, latency monitoring, and trace replay. Some directions to explore:

**Connect to a real model.** Replace `callModel` with a real OpenAI or Anthropic SDK call. The span recording stays the same — you just fill `PromptTokens` and `CompTokens` from `resp.Usage`.

**Add the HTTP server.** Set `HTTPServer: &tokentrace.HTTPServerConfig{Addr: ":9090"}` in the config. Run the program and `curl http://localhost:9090/metrics/cost?window=1h` to see aggregated cost.

**Set a cost alert.** See the [Cost and Quality Alerts tutorial](/tokentrace/tutorials/cost-alerts/) to get notified when spending crosses a threshold.

**Instrument an agent loop.** See [Instrumenting an Agent Loop](/tokentrace/docs/agent-instrumentation/) for a guide to adding step-level tracing to multi-call workflows.

</div>
