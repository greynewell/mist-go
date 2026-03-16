---
title: Go API
description: Full Go API reference for tokentrace — Tracer, Span, Transport interfaces, creating a tracer, recording spans, async vs sync flushing, and testing with a mock transport.
---

# Go API

This page documents the full public Go API for tokentrace. All types and functions are in the `github.com/greynewell/tokentrace` package.

## Tracer

`Tracer` is the main entry point. Create one per application (or per service in a multi-service binary). It is safe for concurrent use.

```go
type Tracer struct { /* unexported */ }

// New creates a Tracer from a Config.
func New(cfg Config) *Tracer

// NewFromFile creates a Tracer from a tokentrace.yml file.
func NewFromFile(path string) (*Tracer, error)

// NewFromReader creates a Tracer from an io.Reader containing YAML.
func NewFromReader(r io.Reader) (*Tracer, error)

// Start begins a new trace and returns a Trace handle.
// The name identifies the operation (e.g., "summarize-document").
func (t *Tracer) Start(name string) *Trace

// FromContext restores a Trace handle from a context previously
// set by trace.Context(). Returns nil if no trace context is present.
func (t *Tracer) FromContext(ctx context.Context) *Trace

// Flush blocks until all buffered spans have been delivered.
// Call this before os.Exit or during graceful shutdown.
func (t *Tracer) Flush(ctx context.Context) error

// Shutdown flushes all spans and shuts down the HTTP server (if running).
func (t *Tracer) Shutdown(ctx context.Context) error

// Version returns the tokentrace version string.
func (t *Tracer) Version() string
```

## Config

```go
type Config struct {
    // Transport is required. Use StdoutTransport(), FileTransport(),
    // HTTPTransport(), or MultiTransport().
    Transport Transport

    // Alerts is an optional list of alert rules.
    Alerts []AlertRule

    // CustomMetrics defines additional metrics derived from span attributes.
    CustomMetrics []MetricDef

    // Pricing overrides built-in per-model pricing.
    Pricing map[string]CostRate

    // HTTPServer enables the HTTP API server. Nil means disabled.
    HTTPServer *HTTPServerConfig

    // Retention controls in-memory span retention.
    Retention RetentionConfig
}

type HTTPServerConfig struct {
    Addr           string
    MetricsPath    string
    PrometheusPath string
    IngestPath     string
    HealthPath     string
    ReadTimeout    time.Duration
    WriteTimeout   time.Duration
    AuthToken      string
}

type RetentionConfig struct {
    // MemoryWindow is how far back the in-process aggregator retains spans.
    // Default: 7 * 24 * time.Hour.
    MemoryWindow time.Duration
}
```

## Trace

A `Trace` represents one logical operation (a single trace context). Obtain a `Trace` from `tracer.Start()`.

```go
type Trace struct { /* unexported */ }

// ID returns the trace ID (UUID v4) assigned at Start().
func (t *Trace) ID() string

// Record adds a span to the trace.
func (t *Trace) Record(span Span)

// RecordError records a span with status StatusError and the given error.
// Equivalent to calling Record with Status: StatusError and Error: err.Error().
func (t *Trace) RecordError(err error)

// End marks the trace as complete and flushes spans to the transport.
// For synchronous transports (StdoutTransport, FileTransport with Sync: true),
// End blocks until all spans are written.
// For asynchronous transports (FileTransport with buffering, HTTPTransport),
// End enqueues the spans and returns immediately.
func (t *Trace) End()

// EndAndFlush marks the trace as complete and blocks until all spans
// are fully delivered to the sink, including retries if applicable.
// Use this before process exit or in tests.
func (t *Trace) EndAndFlush(ctx context.Context) error

// Context returns a context.Context carrying the trace ID, suitable for
// passing to other functions that call tracer.FromContext().
func (t *Trace) Context() context.Context

// WithContext stores the trace context in ctx and returns the new ctx.
func (t *Trace) WithContext(ctx context.Context) context.Context
```

## Span

See [Traces & Spans](/tokentrace/docs/traces/) for the full field reference. Key constructors and helpers:

```go
// Cost computes the cost in USD for a model and token usage.
// Uses the built-in pricing table. Returns 0 if the model is unknown.
func Cost(model string, usage Usage) float64

// CostWithRate computes cost using a custom CostRate.
func CostWithRate(rate CostRate, promptTokens, compTokens int) float64

type Usage struct {
    PromptTokens     int
    CompletionTokens int
}

type CostRate struct {
    PromptPer1M     float64  // USD per 1M prompt tokens
    CompletionPer1M float64  // USD per 1M completion tokens
}
```

## Transport interface

```go
type Transport interface {
    Send(ctx context.Context, spans []Span) error
    Flush(ctx context.Context) error
    Close() error
}
```

### Built-in transports

```go
// StdoutTransport writes spans as JSON to stdout.
func StdoutTransport() Transport
func StdoutTransportWith(opts StdoutOptions) Transport

// FileTransport writes spans as JSONL to a file.
func FileTransport(path string) Transport
func FileTransportWith(opts FileOptions) Transport

// HTTPTransport POSTs spans to an HTTP endpoint.
func HTTPTransport(endpoint string) Transport
func HTTPTransportWith(opts HTTPOptions) Transport

// MultiTransport fans out to multiple transports.
func MultiTransport(transports ...Transport) Transport

// NoopTransport discards all spans.
func NoopTransport() Transport

// BufferTransport stores spans in memory. Used for testing.
func BufferTransport() *Buffer
```

### BufferTransport for testing

`BufferTransport` stores spans in memory and provides assertions. Use it in tests to verify that your instrumented code emits the expected spans without any I/O.

```go
func TestSummarizer(t *testing.T) {
    buf := tokentrace.BufferTransport()
    tracer := tokentrace.New(tokentrace.Config{
        Transport: buf,
    })

    s := NewSummarizer(tracer)
    _, err := s.Summarize("Some document text...")
    if err != nil {
        t.Fatal(err)
    }

    // Flush synchronously so all spans are in the buffer
    ctx := context.Background()
    if err := tracer.Flush(ctx); err != nil {
        t.Fatal(err)
    }

    spans := buf.Spans()
    if len(spans) != 1 {
        t.Fatalf("expected 1 span, got %d", len(spans))
    }

    span := spans[0]
    if span.Model != "gpt-4o" {
        t.Errorf("expected model gpt-4o, got %s", span.Model)
    }
    if span.Status != tokentrace.StatusOK {
        t.Errorf("expected status ok, got %s", span.Status)
    }
    if span.PromptTokens == 0 {
        t.Error("expected non-zero prompt tokens")
    }
    if span.Cost == 0 {
        t.Error("expected non-zero cost")
    }
}
```

`Buffer` methods:

```go
type Buffer struct { /* unexported */ }

// Spans returns all spans received so far, in order.
func (b *Buffer) Spans() []Span

// Len returns the number of spans received.
func (b *Buffer) Len() int

// Reset clears all stored spans.
func (b *Buffer) Reset()

// Last returns the most recently received span.
// Returns zero value if no spans have been received.
func (b *Buffer) Last() Span

// Find returns the first span matching the given predicate.
func (b *Buffer) Find(fn func(Span) bool) (Span, bool)
```

## Budget helper

`Budget` accumulates cost across spans and returns an error when a limit is reached. Use it to enforce a per-workflow cost ceiling.

```go
type Budget struct { /* unexported */ }

// NewBudget creates a budget with a maximum spend in USD.
func NewBudget(maxUSD float64) *Budget

// Add records a span cost and returns an error if the budget is exceeded.
// Returns nil if the budget is not exceeded.
func (b *Budget) Add(span Span) error

// Spent returns the total cost accumulated so far.
func (b *Budget) Spent() float64

// Remaining returns the remaining budget.
func (b *Budget) Remaining() float64

// Exceeded reports whether the budget has been exceeded.
func (b *Budget) Exceeded() bool
```

Example — enforcing a $0.10 budget on an agent run:

```go
budget := tokentrace.NewBudget(0.10)
trace := tracer.Start("agent-run")

for {
    resp, _ := callModel(buildPrompt(state))
    span := tokentrace.Span{
        Model:        "gpt-4o",
        PromptTokens: resp.Usage.PromptTokens,
        CompTokens:   resp.Usage.CompletionTokens,
        LatencyMs:    elapsed.Milliseconds(),
        Cost:         tokentrace.Cost("gpt-4o", resp.Usage),
        Status:       tokentrace.StatusOK,
    }
    trace.Record(span)

    if err := budget.Add(span); err != nil {
        // Budget exceeded — stop the loop
        trace.End()
        return nil, fmt.Errorf("agent budget exceeded: %w", err)
    }

    if isComplete(resp.Content) {
        break
    }
    state = advance(state, resp.Content)
}

trace.End()
```

## Async vs sync flushing

By default, `trace.End()` is non-blocking. Spans are enqueued and delivered by a background goroutine. This is the right behavior for production services — inference calls should not block on observability I/O.

In tests, you need spans to be fully delivered before making assertions. Use one of:

```go
// Option 1: EndAndFlush blocks until all spans are delivered.
if err := trace.EndAndFlush(ctx); err != nil {
    t.Fatal(err)
}

// Option 2: Flush the tracer after End().
trace.End()
if err := tracer.Flush(ctx); err != nil {
    t.Fatal(err)
}

// Option 3: Use BufferTransport, which is always synchronous.
buf := tokentrace.BufferTransport()
tracer := tokentrace.New(tokentrace.Config{Transport: buf})
```

For FileTransport with `Sync: true`, every `trace.End()` call blocks until the span is written and fsynced to disk. This is safe but adds I/O latency to every trace — not recommended for production unless durability is required.

## Next steps

- [Traces & Spans](/tokentrace/docs/traces/) — Span struct field reference.
- [Transports](/tokentrace/docs/transports/) — Transport configuration options.
- [Agent Instrumentation](/tokentrace/docs/agent-instrumentation/) — Practical patterns for agent loops.
