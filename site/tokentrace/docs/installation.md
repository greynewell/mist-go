---
title: Installation
description: Install the tokentrace Go package, verify the installation, and optionally start the HTTP metrics server.
---

# Installation

tokentrace is distributed as a Go module. There are no system dependencies, no pre-compiled binaries to download, and no background process to run.

## Requirements

- Go 1.21 or later
- A Go module (`go.mod`) in your project

## Install the package

```
go get github.com/greynewell/tokentrace
```

This adds tokentrace to your `go.mod` and downloads the module. The package has no external dependencies — `go get` will not pull in any third-party libraries.

## Verify

Confirm the module is present:

```
go list -m github.com/greynewell/tokentrace
```

Expected output:

```
github.com/greynewell/tokentrace v0.x.y
```

Compile a minimal program to confirm the import works:

```go
package main

import (
    "fmt"
    "github.com/greynewell/tokentrace"
)

func main() {
    t := tokentrace.New(tokentrace.Config{
        Transport: tokentrace.StdoutTransport(),
    })
    fmt.Println("tokentrace version:", t.Version())
}
```

## Updating

To update to the latest release:

```
go get github.com/greynewell/tokentrace@latest
go mod tidy
```

## Optional: HTTP server setup

tokentrace can serve a metrics and ingestion HTTP API from within your application process. Enable it by setting `HTTPServer` in the config:

```go
tracer := tokentrace.New(tokentrace.Config{
    Transport: tokentrace.FileTransport("./traces.jsonl"),
    HTTPServer: &tokentrace.HTTPServerConfig{
        Addr:            ":9090",
        MetricsPath:     "/metrics",
        PrometheusPath:  "/metrics/prometheus",
        IngestPath:      "/spans",
        ReadTimeout:     5 * time.Second,
        WriteTimeout:    10 * time.Second,
    },
})
```

The HTTP server starts in the background when `tokentrace.New` is called. It does not block the main goroutine. Shut it down cleanly by calling `tracer.Shutdown(ctx)`.

If you prefer to run the HTTP server as a separate process (for example, as a sidecar that receives spans from multiple application instances), see the [HTTP API](/tokentrace/docs/http-api/) reference.

## Next steps

- [Quick Start](/tokentrace/docs/quick-start/) — Instrument your first inference call.
- [Configuration](/tokentrace/docs/config/) — Full `tokentrace.yml` schema reference.
- [Transports](/tokentrace/docs/transports/) — Choose and configure a transport for your environment.
