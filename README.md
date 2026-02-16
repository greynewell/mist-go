# mist-go

Shared library for the MIST stack. Zero external dependencies.

[![CI](https://github.com/greynewell/mist-go/actions/workflows/ci.yml/badge.svg)](https://github.com/greynewell/mist-go/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Zero Dependencies](https://img.shields.io/badge/dependencies-zero-brightgreen)](go.mod)

## Install

```bash
go get github.com/greynewell/mist-go
```

## Tools

| Repo | What |
|------|------|
| [matchspec](https://github.com/greynewell/matchspec) | Eval framework |
| [infermux](https://github.com/greynewell/infermux) | Inference router |
| [schemaflux](https://github.com/greynewell/schemaflux) | Data compiler |
| [tokentrace](https://github.com/greynewell/tokentrace) | Observability |

## Packages

| Package | What |
|---------|------|
| `protocol` | Message envelope, types, versioning, typed payloads |
| `transport` | HTTP, file, stdio, channel transports |
| `cli` | Subcommands, typed flags, help generation |
| `config` | TOML parser, env var overrides, validation |
| `output` | JSON lines, aligned tables |
| `server` | HTTP server, graceful shutdown |
| `errors` | Structured error codes, JSON marshaling |
| `trace` | Distributed tracing, W3C Trace Context |
| `logging` | Structured JSON logging |
| `retry` | Exponential backoff with jitter |
| `health` | Liveness/readiness probes |
| `checkpoint` | Durable progress checkpointing |
| `lifecycle` | Startup/shutdown orchestration |
| `circuitbreaker` | Circuit breaker |
| `metrics` | Counters, gauges, histograms |
| `parallel` | Worker pool, rate limiting, backpressure |
| `resource` | Memory/goroutine limits |
| `platform` | Cross-platform file locking |
| `misttest` | Test helpers |

## Usage

```go
a, b := transport.NewChannelPair(64)
defer a.Close()
defer b.Close()

msg := protocol.New("myapp", "eval.run", map[string]any{
    "suite": "swe-bench-lite",
    "samples": 10,
})
a.Send(context.Background(), msg)
got, _ := b.Receive(context.Background())
```

Transports are URL-addressed:

```go
t, _ := transport.Dial("http://localhost:8081")
t, _ := transport.Dial("file:///tmp/data.jsonl")
t, _ := transport.Dial("stdio://")
t, _ := transport.Dial("chan://")
```

## Test

```bash
go test ./...                                        # 652 tests
go test -race ./...                                  # race detection
go test -fuzz=FuzzUnmarshal -fuzztime=30s ./protocol/
go test -fuzz=FuzzParseTOML -fuzztime=30s ./config/
```

Architecture: [ARCHITECTURE.md](ARCHITECTURE.md)
