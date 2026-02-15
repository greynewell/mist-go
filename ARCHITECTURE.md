# mist-go

Shared library for the MIST stack: **M**atchSpec, **I**nferMux, **S**chemaFlux, **T**okenTrace.

Zero external dependencies. All packages use only the Go standard library.

## Packages

| Package | Purpose |
|---------|---------|
| `cli` | Subcommand framework with flag parsing and help generation |
| `config` | Zero-dep TOML parser with environment variable overrides |
| `output` | JSON lines and aligned table formatting |
| `server` | HTTP server with graceful shutdown |
| `protocol` | MIST message envelope, types, and typed payloads |
| `transport` | Communication layer: HTTP, file, stdio, and in-process channels |

## Communication Model

MIST tools communicate via a universal message envelope carried over
pluggable transports. The same tool code works in every environment —
local dev, Docker Compose, Kubernetes, embedded in a single binary,
or piped together on the command line.

### Message Envelope

Every inter-tool message uses the same structure:

```json
{
  "version": "1",
  "id": "a1b2c3...",
  "source": "infermux",
  "type": "trace.span",
  "timestamp_ns": 1700000000000000000,
  "payload": { ... }
}
```

### Message Types

| Type | Source | Target | Purpose |
|------|--------|--------|---------|
| `data.entities` | SchemaFlux | MatchSpec | Batch of compiled entities |
| `data.schema` | SchemaFlux | Any | Schema definition |
| `infer.request` | MatchSpec | InferMux | LLM inference request |
| `infer.response` | InferMux | MatchSpec | LLM inference response |
| `eval.run` | Orchestrator | MatchSpec | Start evaluation |
| `eval.result` | MatchSpec | Any | Evaluation outcome |
| `trace.span` | Any | TokenTrace | Trace span (MTTP) |
| `trace.alert` | TokenTrace | Any | Quality/cost/latency alert |
| `health.ping` | Any | Any | Liveness check |
| `health.pong` | Any | Any | Liveness response |

### Transports

All transports implement the same interface:

```go
type Transport interface {
    Send(ctx context.Context, msg *protocol.Message) error
    Receive(ctx context.Context) (*protocol.Message, error)
    Close() error
}
```

Create a transport from a URL:

```go
t, _ := transport.Dial("http://localhost:8081")     // HTTP POST
t, _ := transport.Dial("file:///tmp/traces.jsonl")   // JSON lines
t, _ := transport.Dial("stdio://")                   // Unix pipes
t, _ := transport.Dial("chan://")                     // In-process
```

### Deployment Patterns

**Local development** — all tools on localhost, HTTP transports:

```toml
[services]
infermux = "http://localhost:8081"
matchspec = "http://localhost:8082"
tokentrace = "http://localhost:8083"
```

**Docker Compose** — tools as containers on a shared network:

```toml
[services]
infermux = "http://infermux:8080"
matchspec = "http://matchspec:8080"
tokentrace = "http://tokentrace:8080"
```

**CI/CD pipeline** — tools run sequentially, file transport:

```bash
schemaflux build --output file:///tmp/data.jsonl
matchspec run --data file:///tmp/data.jsonl --results file:///tmp/results.jsonl
```

**Unix pipes** — tools composed on the command line:

```bash
schemaflux build --output stdio | matchspec run --input stdio
```

**Embedded** — tools as goroutines in a single binary:

```go
a, b := transport.NewChannelPair(256)
go infermux.Run(a)
go matchspec.Run(b)
```

## Token Trace Protocol (MTTP)

TokenTrace defines its own lightweight trace format rather than
implementing the full OpenTelemetry spec. Traces are JSON objects
with token-level metrics:

```json
{
  "trace_id": "abc123",
  "span_id": "s1",
  "parent_id": "",
  "operation": "inference",
  "start_ns": 1700000000000000000,
  "end_ns": 1700000000500000000,
  "status": "ok",
  "attrs": {
    "model": "claude-sonnet-4-5-20250929",
    "provider": "anthropic",
    "tokens_in": 150,
    "tokens_out": 500,
    "cost_usd": 0.003
  }
}
```

An OpenTelemetry bridge is provided as a separate application for
environments that require OTel compatibility.

## Tool Integration Map

```
SchemaFlux ──data.entities──→ MatchSpec
                                  │
                            infer.request
                                  │
                                  ↓
                              InferMux
                                  │
                            infer.response
                                  │
                                  ↓
                              MatchSpec ──eval.result──→ TokenTrace
                                  │
                             trace.span
                                  │
                                  ↓
InferMux ────trace.span────→ TokenTrace ──trace.alert──→ Any
```

## Design Principles

1. **Zero external dependencies.** Only the Go standard library.
2. **Transport-agnostic.** Same code works over HTTP, files, pipes, or channels.
3. **JSON everywhere.** Messages are JSON. Config is TOML. Output is JSON or tables.
4. **URL-addressed services.** Tools find each other via URLs in config or env vars.
5. **Standalone processes.** Tools compose over network and filesystem, not shared memory.

## Domain Portfolio

| Tool | Domain | Role |
|------|--------|------|
| MatchSpec | matchspec.dev | Evaluation framework |
| InferMux | infermux.dev | Inference routing |
| SchemaFlux | schemaflux.dev | Structured data compiler |
| TokenTrace | tokentrace.dev | Production observability |
| MIST Stack | miststack.dev | Umbrella |
| EDD | evaldriven.org | Methodology manifesto |
| Personal | greynewell.com | Anchor blog |
| Business | agentlab.us | Consulting entity |
