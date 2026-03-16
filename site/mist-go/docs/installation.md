---
title: Installation
description: Add mist-go to your Go module — go get, go.mod requirements, module verification, and importing individual packages.
---

# Installation

mist-go is a standard Go module. Add it to any Go project with `go get`. No system dependencies, no C libraries, no Docker setup required.

## Requirements

- Go 1.24 or later (mist-go uses `math/rand/v2`, `log/slog`, and generic type parameters)
- A Go module (your project must have a `go.mod` file)

## Add the module

```bash
go get github.com/greynewell/mist-go
```

This adds mist-go to your `go.mod` and downloads the module. Because mist-go has no external dependencies, `go get` adds a single entry to your `go.mod` and nothing to `go.sum` beyond the module's own checksums.

Your `go.mod` will include:

```
require github.com/greynewell/mist-go v0.1.0
```

## Verify the module

Verify the module's checksums against the Go checksum database:

```bash
go mod verify
```

To inspect what was downloaded:

```bash
go mod download -json github.com/greynewell/mist-go
```

## Import individual packages

mist-go packages are independent. Import only the packages you use:

```go
import (
    "github.com/greynewell/mist-go/protocol"
    "github.com/greynewell/mist-go/transport"
    "github.com/greynewell/mist-go/trace"
    "github.com/greynewell/mist-go/metrics"
    "github.com/greynewell/mist-go/config"
    "github.com/greynewell/mist-go/health"
    "github.com/greynewell/mist-go/lifecycle"
    "github.com/greynewell/mist-go/circuitbreaker"
    "github.com/greynewell/mist-go/checkpoint"
    "github.com/greynewell/mist-go/parallel"
    "github.com/greynewell/mist-go/retry"
    "github.com/greynewell/mist-go/logging"
    "github.com/greynewell/mist-go/errors"
    "github.com/greynewell/mist-go/server"
    "github.com/greynewell/mist-go/cli"
    "github.com/greynewell/mist-go/output"
    "github.com/greynewell/mist-go/resource"
    "github.com/greynewell/mist-go/platform"
)
```

The Go toolchain only compiles packages you actually import, so importing a subset has no overhead from the rest.

## Minimal example

A working program that sends a message over HTTP:

```go
package main

import (
    "context"
    "log"

    "github.com/greynewell/mist-go/protocol"
    "github.com/greynewell/mist-go/transport"
)

func main() {
    ctx := context.Background()

    // Create a typed message.
    msg, err := protocol.New("my-tool", protocol.TypeHealthPing, protocol.HealthPing{
        From: "my-tool",
    })
    if err != nil {
        log.Fatal(err)
    }

    // Dial a transport from a URL.
    t, err := transport.Dial("http://localhost:8080")
    if err != nil {
        log.Fatal(err)
    }
    defer t.Close()

    if err := t.Send(ctx, msg); err != nil {
        log.Fatal(err)
    }
}
```

## Using a specific version

To pin to a specific release:

```bash
go get github.com/greynewell/mist-go@v0.1.0
```

To update to the latest release:

```bash
go get -u github.com/greynewell/mist-go
```

## Vendoring

mist-go works with `go mod vendor`:

```bash
go mod vendor
```

Because there are no external dependencies, vendoring adds only the mist-go source tree itself. The vendor directory will contain `github.com/greynewell/mist-go` and nothing else.

## Next steps

- [Overview](/mist-go/docs/overview/) — Architecture, package dependency graph, and design principles.
- [protocol](/mist-go/docs/protocol/) — Message envelope, type constants, and payload types.
- [transport](/mist-go/docs/transport/) — HTTP, file, stdio, and channel transports.
