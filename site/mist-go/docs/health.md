---
title: health & lifecycle
description: The health and lifecycle packages — HealthChecker, liveness and readiness probes, graceful shutdown with drain groups and LIFO hooks, and signal handling.
---

# health & lifecycle

Import paths:
- `github.com/greynewell/mist-go/health`
- `github.com/greynewell/mist-go/lifecycle`

The `health` package provides HTTP health check handlers for Kubernetes probes and load balancer checks. The `lifecycle` package handles graceful startup and shutdown: signal handling, in-flight work draining, and cleanup hook registration. Every MIST tool uses both.

---

## health

### Creating a health handler

```go
h := health.New("matchspec", "1.0.0")
```

`New` takes the tool name and version. These appear in every response body.

### Registering dependency checks

A `CheckFunc` is any function that returns nil for healthy or an error for unhealthy. Register named checks that run during readiness probes:

```go
// Check that the inference backend is reachable.
h.AddCheck("infermux", func() error {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()
    return pingInfermux(ctx)
})

// Check that the checkpoint directory is writable.
h.AddCheck("checkpoint-dir", func() error {
    path := filepath.Join(cfg.CheckpointDir, ".healthcheck")
    if err := os.WriteFile(path, []byte("ok"), 0600); err != nil {
        return fmt.Errorf("checkpoint dir not writable: %w", err)
    }
    os.Remove(path)
    return nil
})
```

### Mounting health endpoints

```go
mux := http.NewServeMux()
mux.Handle("GET /healthz", h.Liveness())
mux.Handle("GET /readyz", h.Readiness())
```

### Liveness (`/healthz`)

The liveness handler always returns `200 OK` while the process is running. It does not run dependency checks. Use this for Kubernetes `livenessProbe` and load balancer pings.

```json
{
  "status": "ok",
  "tool": "matchspec",
  "version": "1.0.0",
  "uptime": "3h42m18s"
}
```

### Readiness (`/readyz`)

The readiness handler runs all registered `CheckFunc` functions. If all pass, it returns `200 OK`. If any fail, it returns `503 Service Unavailable`. Use this for Kubernetes `readinessProbe` — it tells the scheduler when the pod is ready to accept traffic.

```json
{
  "status": "ok",
  "tool": "matchspec",
  "version": "1.0.0",
  "uptime": "3h42m18s",
  "checks": {
    "infermux": "ok",
    "checkpoint-dir": "ok"
  }
}
```

On failure:

```json
{
  "status": "degraded",
  "tool": "matchspec",
  "version": "1.0.0",
  "uptime": "3h42m18s",
  "checks": {
    "infermux": "dial tcp: connection refused",
    "checkpoint-dir": "ok"
  }
}
```

### Manual readiness control

For controlled rollouts, you can temporarily mark a tool as not ready:

```go
// Take the tool out of rotation before maintenance.
h.SetReady(false)
// ... do maintenance work ...
h.SetReady(true)
```

`SetReady(false)` causes the readiness endpoint to return `503` without running checks, allowing you to drain traffic before an operation that would affect service quality.

---

## lifecycle

### lifecycle.Run

`Run` is the entry point for all MIST tools. It wraps your main logic with signal handling and graceful shutdown:

```go
func main() {
    err := lifecycle.Run(func(ctx context.Context) error {
        // Your application logic here.
        // ctx is cancelled when SIGTERM or SIGINT is received.
        return server.ListenAndServe()
    })
    if err != nil && !errors.Is(err, http.ErrServerClosed) {
        log.Printf("exit: %v", err)
        os.Exit(1)
    }
}
```

When `Run` is called, it:
1. Creates a context that cancels on `SIGTERM` or `SIGINT`
2. Runs your function in a goroutine
3. Waits for either the function to return or a signal to arrive
4. Drains in-flight work (via `DrainGroup`)
5. Runs shutdown hooks in reverse registration order (via `OnShutdown`)
6. Returns the first error encountered

Panics in your function are recovered and returned as errors.

### Shutdown hooks

`OnShutdown` registers a function to run during the shutdown phase. Hooks are run in reverse registration order (LIFO, like `defer`):

```go
lifecycle.Run(func(ctx context.Context) error {
    db, err := openDatabase(cfg.DSN)
    if err != nil {
        return err
    }
    lifecycle.OnShutdown(ctx, func() error {
        return db.Close()
    })

    cache := openCache()
    lifecycle.OnShutdown(ctx, func() error {
        return cache.Flush()
    })

    // On shutdown: cache.Flush runs first, then db.Close.
    return server.ListenAndServe()
})
```

This mirrors the `defer` ordering convention: register resources in the order you open them, and they close in reverse.

### Drain groups

`DrainGroup` returns a `*sync.WaitGroup` that lifecycle will wait on before running shutdown hooks. Use it to track in-flight work that must complete before cleanup:

```go
lifecycle.Run(func(ctx context.Context) error {
    dg := lifecycle.DrainGroup(ctx)

    for msg := range incoming {
        dg.Add(1)
        go func(m *protocol.Message) {
            defer dg.Done()
            processMessage(m)
        }(msg)
    }
    return nil
})
```

On shutdown, `lifecycle.Run` waits for all drain group WaitGroups to reach zero before running shutdown hooks. The default drain timeout is 15 seconds.

### Configuring timeouts

```go
err := lifecycle.Run(
    func(ctx context.Context) error {
        return server.ListenAndServe()
    },
    lifecycle.WithDrainTimeout(30*time.Second),  // wait up to 30s for in-flight work
    lifecycle.WithShutdownTimeout(15*time.Second), // run hooks within 15s
)
```

If the drain timeout is exceeded, `Run` logs a warning, proceeds to shutdown hooks, and returns the timeout error. If the shutdown hook timeout is exceeded, hooks are interrupted.

### Full example

A complete MIST tool main function:

```go
func main() {
    var cfg AppConfig
    if err := config.Load("matchspec.toml", "MATCHSPEC", &cfg); err != nil {
        log.Fatal(err)
    }

    log := logging.New("matchspec", logging.LevelInfo)
    reg := metrics.NewRegistry()
    h := health.New("matchspec", version)

    err := lifecycle.Run(func(ctx context.Context) error {
        // Set up transport.
        inferTr := transport.NewHTTP(cfg.Infer.URL)
        lifecycle.OnShutdown(ctx, func() error { return inferTr.Close() })

        // Register readiness checks.
        h.AddCheck("infermux", func() error { return pingInfermux(inferTr) })

        // Set up HTTP server.
        srv := server.New(cfg.Server.Addr)
        srv.Handle("GET /healthz", h.Liveness())
        srv.Handle("GET /readyz", h.Readiness())
        srv.Handle("GET /metricsz", reg.Handler())
        lifecycle.OnShutdown(ctx, func() error { return srv.Shutdown(ctx) })

        log.Info(ctx, "starting", "addr", cfg.Server.Addr)
        return srv.ListenAndServe()
    })

    if err != nil {
        log.Error(context.Background(), "exit", "error", err)
        os.Exit(1)
    }
}
```
