---
title: Transports
slug: transports
order: 3
---

# Transports

All transports implement the same interface:

```go
type Transport interface {
    Send(ctx context.Context, msg *protocol.Message) error
    Receive(ctx context.Context) (*protocol.Message, error)
    Close() error
}
```

## Dial

Create any transport from a URL:

```go
t, err := transport.Dial("http://localhost:8081")     // HTTP POST
t, err := transport.Dial("file:///tmp/traces.jsonl")   // JSON lines
t, err := transport.Dial("stdio://")                   // Unix pipes
t, err := transport.Dial("chan://")                     // In-process
```

## HTTP

POSTs messages to a target URL. Start a listener for receiving:

```go
h := transport.NewHTTP("http://peer:8080")
go h.ListenForMessages(":8081")
```

## File

Appends JSON lines for sending, reads lines for receiving. Ideal for
CI/CD pipelines where tools run sequentially:

```bash
schemaflux build --output file:///tmp/data.jsonl
matchspec run --data file:///tmp/data.jsonl
```

## Stdio

Reads from stdin, writes to stdout. Enables Unix pipe composition:

```bash
schemaflux build --output stdio | matchspec run --input stdio
```

## Channel

In-process Go channels. Use `NewChannelPair` for bidirectional
communication between goroutines:

```go
a, b := transport.NewChannelPair(256)
go toolA.Run(a)
go toolB.Run(b)
```
