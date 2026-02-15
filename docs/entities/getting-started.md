---
title: Getting Started
slug: getting-started
order: 1
---

# Getting Started with mist-go

mist-go is the shared Go library for the MIST stack. It provides the
protocol, transports, and utilities shared across MatchSpec, InferMux,
SchemaFlux, and TokenTrace.

## Installation

```bash
go get github.com/greynewell/mist-go@latest
```

## Quick Example

Send a health ping over an in-process channel:

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/greynewell/mist-go/protocol"
    "github.com/greynewell/mist-go/transport"
)

func main() {
    a, b := transport.NewChannelPair(256)
    defer a.Close()
    defer b.Close()

    ctx := context.Background()

    msg, _ := protocol.New("example", protocol.TypeHealthPing,
        protocol.HealthPing{From: "example"})

    if err := a.Send(ctx, msg); err != nil {
        log.Fatal(err)
    }

    got, err := b.Receive(ctx)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("received: %s from %s\n", got.Type, got.Source)
}
```

## Design Principles

1. **Zero external dependencies.** Only the Go standard library.
2. **Transport-agnostic.** Same code works over HTTP, files, pipes, or channels.
3. **JSON everywhere.** Messages are JSON. Config is TOML. Output is JSON or tables.
