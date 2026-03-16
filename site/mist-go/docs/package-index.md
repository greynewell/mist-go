---
title: All 19 Packages
description: Complete reference table of all 19 packages in mist-go — import paths, key types, and descriptions. Plus brief documentation for retry, logging, errors, server, cli, output, resource, platform, and bindings.
---

# All 19 Packages

Complete reference for every package in `github.com/greynewell/mist-go`.

## Package reference table

| Package | Import path | Key types | Description |
|---------|-------------|-----------|-------------|
| `protocol` | `mist-go/protocol` | `Message`, `InferRequest`, `EvalRun`, `TraceSpan` | Message envelope, type constants, and all structured payload types |
| `transport` | `mist-go/transport` | `Transport`, `HTTP`, `File`, `Stdio`, `Channel` | Transport interface and four implementations: HTTP, file, stdio, channel |
| `trace` | `mist-go/trace` | `Span` | Context-based distributed tracing with W3C Trace Context support |
| `metrics` | `mist-go/metrics` | `Registry`, `Counter`, `Gauge`, `Histogram` | Lock-free counters, gauges, and histograms with JSON HTTP handler |
| `config` | `mist-go/config` | `Load`, `ParseTOML`, `Decode` | TOML config loading with environment variable overlay |
| `health` | `mist-go/health` | `Handler`, `CheckFunc` | HTTP liveness and readiness probes with named dependency checks |
| `lifecycle` | `mist-go/lifecycle` | `Run`, `OnShutdown`, `DrainGroup` | Signal handling, graceful shutdown, LIFO hooks, drain groups |
| `circuitbreaker` | `mist-go/circuitbreaker` | `Breaker`, `Config`, `State` | Three-state circuit breaker (Closed/Open/HalfOpen) |
| `checkpoint` | `mist-go/checkpoint` | `Tracker`, `Record` | Incremental checkpointing for resumable long-running jobs |
| `parallel` | `mist-go/parallel` | `Pool`, `Result` | Bounded worker pool: `Map`, `Do`, `FanOut` |
| `retry` | `mist-go/retry` | `Policy`, `Classifier` | Exponential backoff with jitter and retryability classification |
| `logging` | `mist-go/logging` | `Logger` | Trace-aware structured logger built on `log/slog` |
| `errors` | `mist-go/errors` | `Error` | Structured errors with code, cause, and HTTP/exit code mapping |
| `server` | `mist-go/server` | `Server` | Minimal HTTP server with graceful shutdown on interrupt |
| `cli` | `mist-go/cli` | `App`, `Command` | Subcommand framework built on `flag` |
| `output` | `mist-go/output` | `Writer` | JSON-lines and table formatting for CLI output |
| `resource` | `mist-go/resource` | `Limiter`, `MemoryBudget`, `Monitor` | Concurrency limiting, memory budget tracking, resource monitoring |
| `platform` | `mist-go/platform` | — | Cross-platform: OS detection, line ending normalization, file locking |
| `bindings` | `mist-go/bindings/python`, `mist-go/bindings/typescript` | — | Generated client bindings for Python and TypeScript |

---

## Packages with dedicated documentation

The following packages have full documentation pages:

- [protocol](/mist-go/docs/protocol/) — Message envelope and payload types
- [transport](/mist-go/docs/transport/) — HTTP, file, stdio, and channel transports
- [trace](/mist-go/docs/trace/) — Distributed tracing
- [metrics](/mist-go/docs/metrics/) — Counters, gauges, histograms
- [config](/mist-go/docs/config/) — Configuration loading
- [health & lifecycle](/mist-go/docs/health/) — Health checks and graceful shutdown
- [circuitbreaker](/mist-go/docs/circuitbreaker/) — Circuit breaker
- [checkpoint](/mist-go/docs/checkpoint/) — Incremental checkpointing
- [parallel](/mist-go/docs/parallel/) — Worker pool

---

## retry

Import path: `github.com/greynewell/mist-go/retry`

Exponential backoff with jitter. The primary function is `Do(ctx, policy, fn)`:

```go
err := retry.Do(ctx, retry.DefaultPolicy, func(ctx context.Context) error {
    return transport.Send(ctx, msg)
})
```

**Policies:**

```go
// DefaultPolicy: 3 attempts, 100ms initial, 2x backoff, 5s max, 25% jitter.
retry.DefaultPolicy

// AggressivePolicy: 5 attempts, 50ms initial, 2x backoff, 10s max, 25% jitter.
retry.AggressivePolicy

// Custom policy.
policy := retry.Policy{
    MaxAttempts: 5,
    InitialWait: 200 * time.Millisecond,
    MaxWait:     10 * time.Second,
    Multiplier:  2.0,
    Jitter:      0.25,
}
```

**Automatic error classification:**

`DoAuto` uses the `errors` package to classify errors as retryable or permanent. Errors with codes `timeout`, `transport`, `unavailable`, or `rate_limit` are retried; `validation`, `auth`, `not_found`, `conflict` are not:

```go
err := retry.DoAuto(ctx, retry.DefaultPolicy, func(ctx context.Context) error {
    return callInfermux(ctx, req)
})
```

**Custom classifier:**

```go
classifier := func(err error) bool {
    // Only retry on network errors, not application errors.
    var netErr net.Error
    return errors.As(err, &netErr) && netErr.Timeout()
}
err := retry.DoWithClassifier(ctx, retry.DefaultPolicy, classifier, fn)
```

`TotalMaxWait()` returns the worst-case total wait time across all retries for a policy.

---

## logging

Import path: `github.com/greynewell/mist-go/logging`

Trace-aware structured logger built on `log/slog`. Automatically injects `trace_id` and `span_id` from the context into every log entry.

```go
log := logging.New("matchspec", logging.LevelInfo)

// In a handler:
ctx, span := trace.Start(ctx, "eval-run")
log.Info(ctx, "eval started", "suite", "swe-bench", "tasks", 300)
// Output: {"level":"INFO","tool":"matchspec","msg":"eval started","suite":"swe-bench","tasks":300,"trace_id":"a3f2...","span_id":"b1c2..."}
```

**Format options:**

```go
// JSON output (default, for production).
log := logging.New("matchspec", logging.LevelInfo)

// Text output (for development).
log := logging.New("matchspec", logging.LevelInfo,
    logging.WithFormat("text"),
)

// Custom writer.
log := logging.New("matchspec", logging.LevelInfo,
    logging.WithWriter(os.Stdout),
)
```

**Dynamic level adjustment:**

```go
log.SetLevel(logging.LevelDebug) // enable debug logging at runtime
```

**Permanent attributes:**

```go
requestLog := log.With("request_id", reqID, "user", userID)
requestLog.Info(ctx, "processing request")
// includes request_id and user in every entry
```

**slog interop:**

```go
slogLogger := log.Slog() // returns *slog.Logger for libraries that require it
```

---

## errors

Import path: `github.com/greynewell/mist-go/errors`

Structured error type with codes, messages, causes, and metadata. Codes map to HTTP status codes and process exit codes.

```go
// Create structured errors.
err := errors.New(errors.CodeValidation, "suite name is required")
err := errors.Newf(errors.CodeNotFound, "suite %q not found", suiteName)
err := errors.Wrap(errors.CodeTransport, originalErr, "send to infermux failed")

// Add metadata.
err = err.WithMeta("suite", suiteName).WithMeta("task", taskID)

// Check the code.
fmt.Println(errors.Code(err)) // "not_found"

// Map to HTTP status.
http.Error(w, err.Error(), errors.HTTPStatus(errors.Code(err))) // 404

// Map to process exit code.
os.Exit(errors.ExitCode(errors.Code(err))) // 3

// Retryability.
errors.IsRetryable(err) // false (not_found is permanent)
```

**Standard codes:**

| Code | HTTP status | Exit code | Retryable |
|------|-------------|-----------|-----------|
| `internal` | 500 | 1 | yes |
| `timeout` | 504 | 5 | yes |
| `cancelled` | 499 | 130 | — |
| `transport` | 500 | 7 | yes |
| `protocol` | 500 | 8 | no |
| `validation` | 400 | 2 | no |
| `not_found` | 404 | 3 | no |
| `unavailable` | 503 | 6 | yes |
| `rate_limit` | 429 | 9 | yes |
| `auth` | 401 | 4 | no |
| `conflict` | 409 | 10 | no |

**Override retryability:**

```go
err = err.Retriable()  // force retryable regardless of code
err = err.Permanent()  // force non-retryable regardless of code
```

**Error chains:**

```go
err := errors.Wrap(errors.CodeTransport, originalErr, "send failed")
fmt.Println(err.Error()) // "transport: send failed: <original>"

// Unwrap.
var e *errors.Error
if errors.As(err, &e) {
    fmt.Println(e.Code) // "transport"
}
```

---

## server

Import path: `github.com/greynewell/mist-go/server`

Minimal HTTP server with graceful shutdown on `SIGINT`:

```go
srv := server.New(":8080")
srv.Handle("GET /healthz", healthHandler)
srv.Handle("GET /readyz", readyHandler)
srv.Handle("POST /mist", messageHandler)
srv.Handle("GET /metricsz", metricsHandler)

if err := srv.ListenAndServe(); err != nil {
    log.Fatal(err)
}
```

`ListenAndServe` prints the listening address to stderr and blocks until interrupted. On `SIGINT`, it calls `http.Server.Shutdown` with a 5-second timeout.

For more control over shutdown (integration with `lifecycle.Run`), use `net/http.Server` directly.

---

## cli

Import path: `github.com/greynewell/mist-go/cli`

Subcommand framework built on `flag`. `NewApp` creates an application with a built-in `version` command:

```go
app := cli.NewApp("matchspec", version)

runCmd := &cli.Command{
    Name:  "run",
    Usage: "Run an eval suite",
}
runCmd.AddStringFlag("config", "matchspec.toml", "Config file path")
runCmd.AddIntFlag("workers", 8, "Number of parallel workers")
runCmd.Run = func(cmd *cli.Command, args []string) error {
    cfgPath := cmd.GetString("config")
    workers := cmd.GetInt("workers")
    return runSuite(cfgPath, workers)
}
app.AddCommand(runCmd)

os.Exit(func() int {
    if err := app.Execute(os.Args[1:]); err != nil {
        fmt.Fprintln(os.Stderr, err)
        return 1
    }
    return 0
}())
```

Flag types: `AddStringFlag`, `AddIntFlag`, `AddInt64Flag`, `AddFloat64Flag`, `AddBoolFlag`.
Accessors: `GetString`, `GetInt`, `GetInt64`, `GetFloat64`, `GetBool`.

---

## output

Import path: `github.com/greynewell/mist-go/output`

JSON and table formatting for CLI output:

```go
w := output.New("json") // or "table"

// JSON output (one object per line).
w.JSON(result)

// Table output.
w.Table(
    []string{"SUITE", "SCORE", "STATUS"},
    [][]string{
        {"swe-bench-verified", "0.594", "PASS"},
        {"swe-bench-lite", "0.621", "PASS"},
    },
)

// Write errors to stderr.
output.Error("config error: %v", err)
```

JSON uses `json.Encoder` with HTML escaping disabled. Tables use `text/tabwriter` for column alignment.

---

## resource

Import path: `github.com/greynewell/mist-go/resource`

Concurrency limiting and memory budget tracking:

```go
// Semaphore-style limiter with context support.
limiter := resource.NewLimiter("infer-calls", 10)

if err := limiter.Acquire(ctx); err != nil {
    return err // context was cancelled
}
defer limiter.Release()
callInfermux(ctx, req)

// Non-blocking tryacquire.
if limiter.TryAcquire() {
    defer limiter.Release()
    callInfermux(ctx, req)
}

// Run in a goroutine with limit enforcement.
limiter.Go(ctx, func() {
    callInfermux(ctx, req)
})

// Memory budget.
budget := resource.NewMemoryBudget("eval-results", 512*1024*1024) // 512MB
if !budget.Reserve(estimatedSize) {
    return fmt.Errorf("memory budget exceeded")
}
defer budget.Release(estimatedSize)

// Snapshot current runtime resource usage.
snap := resource.TakeSnapshot()
fmt.Printf("heap: %d bytes, goroutines: %d, CPUs: %d\n",
    snap.HeapBytes, snap.Goroutines, snap.NumCPU)
```

`Monitor` aggregates multiple limiters and budgets for a unified status view. `resource.HeapUsage()` and `resource.GoroutineCount()` expose runtime stats.

---

## platform

Import path: `github.com/greynewell/mist-go/platform`

Cross-platform abstractions:

```go
platform.OS()           // "darwin", "linux", "windows"
platform.Arch()         // "arm64", "amd64"
platform.IsWindows()    // false on Unix

// Line ending normalization (critical for file transport on Windows).
normalized := platform.NormalizeLineEndings(data) // \r\n → \n
native := platform.ToPlatformLineEndings(data)    // \n → \r\n on Windows

platform.PlatformLineEnding() // "\n" on Unix, "\r\n" on Windows
```

The `platform` package also provides file locking via `Lock` and `Unlock` with separate `lock_unix.go` and `lock_windows.go` implementations (using `flock` on Unix and `LockFileEx` on Windows).

---

## bindings

Import paths:
- `github.com/greynewell/mist-go/bindings/python` (Python package at `bindings/python/`)
- `github.com/greynewell/mist-go/bindings/typescript` (TypeScript package at `bindings/typescript/`)

Generated client bindings that implement the MIST message protocol and transport interface in Python and TypeScript. These allow non-Go services to participate in the MIST stack — sending and receiving typed messages over HTTP or stdio without depending on the Go runtime.

Both bindings implement:
- Message envelope construction and parsing
- Transport abstractions (HTTP and stdio)
- Typed payload classes/interfaces for all standard message types
- Version negotiation

See `bindings/python/README.md` and `bindings/typescript/README.md` for language-specific usage.
