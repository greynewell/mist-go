---
title: Overview
description: Architecture of mist-go — the 19 packages, the protocol-first design, the transport abstraction, and how the packages compose into tools like matchspec and infermux.
---

# Overview

mist-go is the shared Go library that all MIST stack tools are built on. It provides the protocol definition, transport layer, observability primitives, configuration loading, lifecycle management, and a set of production-hardened utilities — all with zero external dependencies. The 19 packages in mist-go are organized so each can be used independently: you can pull in just `transport` and `protocol` for a lightweight message-passing component, or use the full set to build a production-grade MIST tool.

## Design principles

**Protocol-first.** Every message that crosses a tool boundary uses the `protocol.Message` envelope. The envelope is the same regardless of what the message contains: a version string, a random ID, a source identifier, a type string, a nanosecond timestamp, a JSON payload, and an optional CRC32 checksum. This uniformity allows routing, logging, and auditing at the transport layer without knowledge of payload contents.

**Transport-agnostic.** The `Transport` interface is three methods. The four built-in implementations (HTTP, file, stdio, channel) are interchangeable — application code calls `Send` and `Receive` and does not know or care which transport is in use. Switching from HTTP to stdio for a batch pipeline, or to an in-process channel for a test, requires changing one line.

**Zero external dependencies.** `go.mod` lists no third-party packages. Every capability — TOML parsing, structured logging, distributed tracing, lock-free metrics, circuit breaking, W3C Trace Context — is implemented on the Go standard library. This eliminates supply chain risk and version conflict for projects that embed mist-go.

**Composable primitives.** Each package has a narrow scope and a clean interface. `circuitbreaker` wraps any `func(context.Context) error`. `parallel.Map` works over any type. `checkpoint.Step` persists results from any function. The packages compose naturally; they do not force a particular application structure.

**Strong typing throughout.** The protocol layer uses concrete struct types for every payload kind (`InferRequest`, `EvalRun`, `TraceSpan`, etc.) and concrete types for message metadata. There is no `interface{}` in the protocol layer and no stringly-typed configuration API.

## The 19 packages

### Core protocol

**`protocol`** — The message envelope (`Message`), type constants (`TypeInferRequest`, `TypeEvalRun`, etc.), source identifiers, all structured payload types (`InferRequest`, `InferResponse`, `EvalRun`, `EvalResult`, `TraceSpan`, `TraceAlert`, `DataEntities`, `DataSchema`, `HealthPing`, `HealthPong`), and version negotiation. This package has no dependencies on any other mist-go package.

**`transport`** — The `Transport` interface, `Dial` factory, and four implementations: `HTTP`, `File`, `Stdio`, and `Channel`. Also includes `Middleware` for wrapping transports with cross-cutting behavior, `Resilient` for transport-level retry, and `Backpressure` for rate limiting sends.

### Observability

**`trace`** — Context-based distributed tracing. `Span` carries trace ID, span ID, parent ID, operation name, start/end nanoseconds, status, and arbitrary attributes. `Start` creates child spans from context. W3C Trace Context injection and extraction for HTTP headers.

**`metrics`** — `Counter`, `Gauge`, and `Histogram` with lock-free atomic operations. `Registry` for grouping metrics with label support. `Handler` for serving metrics as JSON over HTTP.

**`logging`** — Trace-aware structured logger built on `log/slog`. Automatically injects `trace_id` and `span_id` from context. JSON output for production, text output for development.

### Configuration and lifecycle

**`config`** — TOML parsing (implemented from scratch, no external parser) and struct decoding with environment variable overlay. `Load(path, prefix, &cfg)` reads a TOML file and then applies `PREFIX_FIELDNAME` environment variables on top.

**`health`** — HTTP health check handler. `New(tool, version)` creates a `Handler` with `Liveness()` (`/healthz`) and `Readiness()` (`/readyz`) HTTP handlers. Named dependency checks registered via `AddCheck`.

**`lifecycle`** — Graceful startup and shutdown. `Run(fn)` executes your main function with a context that cancels on `SIGTERM`/`SIGINT`. `OnShutdown` registers LIFO cleanup hooks. `DrainGroup` tracks in-flight work that must complete before hooks run.

### Reliability

**`circuitbreaker`** — Three-state circuit breaker (Closed/Open/HalfOpen). `Do(ctx, fn)` wraps any function. `DoWithFallback` provides an alternative path when the circuit is open. Configurable failure threshold, recovery timeout, and probe concurrency.

**`retry`** — Exponential backoff with jitter. `Do(ctx, policy, fn)` retries on any error. `DoAuto` uses the `errors` package to distinguish retryable from permanent failures. `DefaultPolicy` (3 attempts, 100ms initial, 2x backoff, 25% jitter) and `AggressivePolicy` (5 attempts, 50ms initial) are provided.

**`checkpoint`** — Incremental checkpointing for long-running jobs. `Tracker` persists step results to a JSON-lines file keyed by run ID. Steps that completed on a previous run are skipped automatically on resume. `StepRetry` combines checkpointing with retry.

### Concurrency

**`parallel`** — Worker pool for bounded concurrency. `Map[In, Out]` applies a function to a slice in parallel and returns results in input order. `Do` collects errors. `FanOut` sends one input to multiple functions concurrently.

**`resource`** — Resource limiting and monitoring. `Limiter` is a semaphore with context support and usage tracking. `MemoryBudget` tracks reservations against a byte limit. `Monitor` aggregates limiters and budgets for a unified status view.

### Application framework

**`server`** — Minimal HTTP server with graceful shutdown on interrupt. Wraps `net/http.Server` with a `Handle(pattern, fn)` API and a `ListenAndServe` that handles `SIGINT`.

**`cli`** — Subcommand framework built on `flag`. `App` routes to `Command` instances; each command has its own `FlagSet`. Typed flag accessors (`GetString`, `GetInt`, `GetBool`, etc.).

**`output`** — JSON and table formatting for CLI output. `Writer` dispatches between JSON-lines and `text/tabwriter`-aligned tables.

**`errors`** — Structured error type with code, message, cause chain, and metadata. Standard codes (`CodeInternal`, `CodeTimeout`, `CodeTransport`, etc.) that map to HTTP status codes and process exit codes. `IsRetryable` for classifying errors in retry logic.

### Platform and bindings

**`platform`** — Cross-platform abstractions: OS and architecture detection, line ending normalization, file locking (separate implementations for Unix and Windows).

**`bindings`** — Generated client bindings for Python (`bindings/python`) and TypeScript (`bindings/typescript`) that implement the MIST message protocol and transport interface in those languages.

## Package dependency diagram

```
                          ┌──────────────┐
                          │   protocol   │
                          └──────┬───────┘
                                 │ imported by
           ┌─────────────────────┼──────────────────────────────┐
           │                     │                              │
    ┌──────▼──────┐       ┌──────▼──────┐              ┌───────▼──────┐
    │  transport  │       │    trace    │              │    errors    │
    └──────┬──────┘       └──────┬──────┘              └───────┬──────┘
           │                     │                              │
    ┌──────▼──────┐       ┌──────▼──────┐              ┌───────▼──────┐
    │  resilient  │       │   logging   │              │    retry     │
    │ (transport) │       │  (logging)  │              │   (retry)    │
    └─────────────┘       └─────────────┘              └─────────────-┘
           │                     │
    ┌──────▼──────────────────────▼──────────────────────────────────┐
    │                         metrics                                │
    └────────────────────────────────────────────────────────────────┘
           │
    ┌──────▼───────────────────────────────────────────────────────────────────┐
    │   health · lifecycle · circuitbreaker · checkpoint · parallel · resource │
    └──────────────────────────────────────────────────────────────────────────┘
           │
    ┌──────▼──────────────────────────────────────────────┐
    │          config · server · cli · output · platform  │
    └─────────────────────────────────────────────────────┘

        ↑ all of the above are used by the MIST stack tools ↑
    matchspec · infermux · schemaflux · tokentrace
```

The key structural rules:
- `protocol` has no mist-go dependencies. It is the root of the dependency graph.
- `transport` imports `protocol` only.
- `trace` imports nothing from mist-go.
- `logging` imports `trace` to inject trace context into log entries.
- `retry` imports `errors` to classify retryable vs. permanent failures.
- Everything else imports `protocol`, `errors`, or both. No circular dependencies.

## How tools are built from these packages

A MIST stack tool is typically composed of:

1. **`protocol` + `transport`** — define what messages the tool sends and receives, and how they travel.
2. **`trace` + `metrics` + `logging`** — instrument every operation that matters for production observability.
3. **`config`** — load configuration from a TOML file with environment variable overrides.
4. **`health` + `lifecycle`** — expose health endpoints and handle graceful shutdown.
5. **`circuitbreaker` + `retry`** — protect downstream calls from cascading failures.
6. **`server` + `cli`** — expose an HTTP API and a CLI.

matchspec, for example, uses `parallel.Map` to run eval harnesses concurrently, `checkpoint` to resume interrupted eval runs, `circuitbreaker` to protect calls to infermux, and `transport.NewChannelPair` in its test suite to run end-to-end tests without a network.

## Versioning

mist-go follows [semver](https://semver.org). The protocol version is separate from the library version and is negotiated at connection time via `protocol.NegotiateVersion`. Currently at protocol version 1. The `go.mod` module path is `github.com/greynewell/mist-go` and requires Go 1.24 or later.
