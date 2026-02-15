# mist-go

Shared Go library for the **MIST stack** — MatchSpec, InferMux, SchemaFlux, TokenTrace.

[![CI](https://github.com/greynewell/mist-go/actions/workflows/ci.yml/badge.svg)](https://github.com/greynewell/mist-go/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Zero Dependencies](https://img.shields.io/badge/dependencies-zero-brightgreen)](https://github.com/greynewell/mist-go/blob/main/go.mod)

Zero external dependencies. Every package uses only the Go standard library.

## What is the MIST stack?

Four tools for evaluation-driven AI development:

| Tool | Domain | Purpose |
|------|--------|---------|
| **MatchSpec** | [matchspec.dev](https://matchspec.dev) | Evaluation framework for AI outputs |
| **InferMux** | [infermux.dev](https://infermux.dev) | Inference routing across providers |
| **SchemaFlux** | [schemaflux.dev](https://schemaflux.dev) | Structured data compiler |
| **TokenTrace** | [tokentrace.dev](https://tokentrace.dev) | Token-level observability |

This repo is the shared foundation — protocol, transport, CLI, config, and all cross-cutting concerns.

## Packages

| Package | Purpose |
|---------|---------|
| `protocol` | Message envelope, types, versioning, and typed payloads |
| `transport` | HTTP, file, stdio, and in-process channel transports |
| `cli` | Subcommand framework with typed flags and help generation |
| `config` | Zero-dep TOML parser with env var overrides and validation |
| `output` | JSON lines and aligned table formatting |
| `server` | HTTP server with graceful shutdown |
| `errors` | Structured error codes with JSON marshaling |
| `trace` | Distributed tracing with W3C Trace Context support |
| `logging` | Structured JSON logging |
| `retry` | Exponential backoff with jitter |
| `health` | Liveness and readiness probes |
| `checkpoint` | Durable progress checkpointing |
| `lifecycle` | Graceful startup/shutdown orchestration |
| `circuitbreaker` | Circuit breaker for external calls |
| `metrics` | Counters, gauges, and histograms |
| `parallel` | Worker pool with rate limiting and backpressure |
| `resource` | Memory and goroutine limits |
| `platform` | Cross-platform file locking |
| `misttest` | Test helpers and assertions |

## Install

```bash
go get github.com/greynewell/mist-go
```

## Quick start

MIST tools communicate via a universal message envelope over pluggable transports:

```go
package main

import (
    "context"
    "fmt"

    "github.com/greynewell/mist-go/protocol"
    "github.com/greynewell/mist-go/transport"
)

func main() {
    // Create an in-process transport pair
    a, b := transport.NewChannelPair(64)
    defer a.Close()
    defer b.Close()

    // Send a message
    msg := protocol.New("myapp", "eval.run", map[string]any{
        "suite": "swe-bench-lite",
        "samples": 10,
    })
    a.Send(context.Background(), msg)

    // Receive it on the other end
    got, _ := b.Receive(context.Background())
    fmt.Printf("Received %s from %s\n", got.Type, got.Source)
}
```

Transports are URL-addressed — same code works everywhere:

```go
t, _ := transport.Dial("http://localhost:8081")    // HTTP
t, _ := transport.Dial("file:///tmp/data.jsonl")   // JSON lines file
t, _ := transport.Dial("stdio://")                 // Unix pipes
t, _ := transport.Dial("chan://")                   // In-process
```

## Tools

Each MIST tool lives in its own repo and depends on mist-go:

| Tool | Repo |
|------|------|
| **MatchSpec** | [greynewell/matchspec](https://github.com/greynewell/matchspec) |
| **InferMux** | [greynewell/infermux](https://github.com/greynewell/infermux) |
| **SchemaFlux** | [greynewell/schemaflux](https://github.com/greynewell/schemaflux) |
| **TokenTrace** | [greynewell/tokentrace](https://github.com/greynewell/tokentrace) |

The meta-CLI dispatches to any tool:

```bash
go run ./cmd/mist         # Meta-CLI (runs any tool)
```

## Testing

```bash
# Run all tests (652 tests)
go test ./...

# With race detection
go test -race ./...

# Fuzz tests
go test -fuzz=FuzzUnmarshal -fuzztime=30s ./protocol/
go test -fuzz=FuzzParseTOML -fuzztime=30s ./config/
```

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md) for the full design — message types, transport model, deployment patterns, and the Token Trace Protocol.

## License

MIT — see [LICENSE](LICENSE) for details.

---

Built by [Grey Newell](https://greynewell.com) | [miststack.dev](https://miststack.dev)
