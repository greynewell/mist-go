---
title: transport
description: The transport package — Transport interface, Dial factory, HTTP, file, stdio, and channel transports, middleware, and writing custom transports.
---

# transport

Import path: `github.com/greynewell/mist-go/transport`

The `transport` package provides the communication layer for MIST tools. Every transport implements the same `Transport` interface, so tools can communicate over HTTP, files, stdin/stdout pipes, or in-process Go channels without changing application code. The implementation is selected at configuration time via `Dial`.

## The Transport interface

```go
type Transport interface {
    Send(ctx context.Context, msg *protocol.Message) error
    Receive(ctx context.Context) (*protocol.Message, error)
    Close() error
}
```

`Send` and `Receive` are both context-aware: passing a cancelled context causes them to return immediately with `ctx.Err()`. All implementations are safe for concurrent use.

The interface is split into `Sender` and `Receiver` for cases where only one direction is needed:

```go
type Sender interface {
    Send(ctx context.Context, msg *protocol.Message) error
}

type Receiver interface {
    Receive(ctx context.Context) (*protocol.Message, error)
}
```

## Dial

`Dial` creates a transport from a URL string. The URL scheme determines which implementation is returned:

```go
// HTTP transport — POST messages to the target URL
t, err := transport.Dial("http://localhost:8081")
t, err := transport.Dial("https://infermux.example.com")

// File transport — append/read JSON lines from a file
t, err := transport.Dial("file:///tmp/messages.jsonl")

// Stdio transport — write to stdout, read from stdin
t, err := transport.Dial("stdio://")

// Channel transport — in-process Go channel with buffer size 256
t, err := transport.Dial("chan://")
```

## HTTP transport

`HTTP` sends messages by POSTing JSON to the target URL and receives messages by listening on a local address.

```go
t := transport.NewHTTP("http://localhost:8081")
```

The HTTP client is configured with:
- 10-second request timeout
- TLS 1.2 minimum
- Connection pooling: up to 10 idle connections, 30-second idle timeout
- HTTP/2 attempted automatically
- Response body discarded after reading status code

**Sending** — `Send` marshals the message to JSON and POSTs it to the target URL with `Content-Type: application/json`. Any 4xx or 5xx response is returned as an error.

**Receiving** — To receive messages, call `ListenForMessages` with a local address. This starts an HTTP server that accepts POST requests to `/mist`:

```go
t := transport.NewHTTP("http://remote-tool:8081")

// Start receiving in a goroutine.
go func() {
    if err := t.ListenForMessages(":9090"); err != nil {
        log.Fatal(err)
    }
}()

// Now Receive blocks until a message arrives on :9090.
msg, err := t.Receive(ctx)
```

The receive server accepts up to 1 MB per request body. If the inbox buffer (256 messages) is full, it responds with `503 Service Unavailable`.

**Closing** — `Close` shuts down the receive server with a 5-second timeout. The send client has no persistent connection to close.

## File transport

`File` reads and writes messages as JSON lines (one message per line) to a regular file. It is designed for batch pipelines and offline workflows where tools run sequentially rather than as concurrent services.

```go
t, err := transport.NewFile("/tmp/eval-messages.jsonl")
```

The path is resolved to an absolute path at construction time. The file is opened lazily: the write handle (append mode) on the first `Send`, the read handle (read from start) on the first `Receive`.

```go
// Writer process: append messages to the file.
for _, example := range dataset {
    msg, _ := protocol.New("matchspec", protocol.TypeEvalRun, example)
    t.Send(ctx, msg)
}
t.Close()

// Reader process: consume messages from the file.
for {
    msg, err := t.Receive(ctx)
    if err != nil {
        break // io.EOF or no more messages
    }
    process(msg)
}
```

The scanner buffer is 1 MB per line, which accommodates large payloads. Binary data in payloads must be base64-encoded since JSON lines are line-delimited.

## Stdio transport

`Stdio` writes messages to stdout and reads from stdin, one JSON line per message. This enables Unix-style piping between MIST tools:

```go
t := transport.NewStdio()
```

```bash
# Pipe schemaflux output directly into matchspec
schemaflux build --output stdio | matchspec run --input stdio
```

`Send` writes to `os.Stdout` with a newline. `Receive` reads from `os.Stdin` line by line. The scanner buffer is 1 MB. `Close` is a no-op (you cannot close stdin/stdout in the usual sense).

## Channel transport

`Channel` is an in-process transport backed by Go channels. Use it when embedding multiple MIST tools in the same binary, or for testing without a network:

```go
// Unidirectional: Send and Receive on the same transport.
t := transport.NewChannel(256) // buffer size 256

// Bidirectional pair: A sends, B receives, and vice versa.
a, b := transport.NewChannelPair(256)
```

`NewChannelPair` creates two linked transports. Sending on `a` delivers to `b.Receive`, and sending on `b` delivers to `a.Receive`. This models a full-duplex connection between two in-process tools.

```go
// In a test: wire two tools together without a network.
a, b := transport.NewChannelPair(64)

go func() {
    // Tool A sends a request.
    msg, _ := protocol.New("tool-a", protocol.TypeHealthPing, protocol.HealthPing{From: "tool-a"})
    a.Send(ctx, msg)

    // Tool A receives the response.
    reply, _ := a.Receive(ctx)
    var pong protocol.HealthPong
    reply.Decode(&pong)
    fmt.Println(pong.From) // "tool-b"
}()

// Tool B handles requests.
req, _ := b.Receive(ctx)
resp, _ := protocol.New("tool-b", protocol.TypeHealthPong, protocol.HealthPong{
    From: "tool-b", Version: "1.0.0",
})
b.Send(ctx, resp)
```

`Send` on a full buffer returns an error immediately (`"channel transport: buffer full"`). `Close` closes the send channel; subsequent sends fail.

## Middleware

The `transport` package includes a `Middleware` type for wrapping transports with cross-cutting behavior (logging, metrics, tracing). Middleware intercepts `Send` and `Receive` calls:

```go
// Wrap a transport with logging and metrics.
base := transport.NewHTTP("http://infermux:8081")
wrapped := transport.NewMiddleware(base,
    transport.WithLogging(logger),
    transport.WithMetrics(registry),
)
```

## Resilient transport

`Resilient` wraps a transport with automatic retry on `Send` failures, using the `retry` package's policy:

```go
t := transport.NewResilient(
    transport.NewHTTP("http://infermux:8081"),
    retry.DefaultPolicy,
)
```

Retries are applied only to send errors. `Receive` is passed through unchanged. This is appropriate for one-way message delivery where idempotent sends are acceptable.

## Writing a custom transport

Implement the `Transport` interface:

```go
type MyTransport struct {
    // your fields
}

func (t *MyTransport) Send(ctx context.Context, msg *protocol.Message) error {
    data, err := msg.Marshal()
    if err != nil {
        return err
    }
    // deliver data
    return nil
}

func (t *MyTransport) Receive(ctx context.Context) (*protocol.Message, error) {
    // block until data available or ctx done
    select {
    case data := <-t.incoming:
        return protocol.Unmarshal(data)
    case <-ctx.Done():
        return nil, ctx.Err()
    }
}

func (t *MyTransport) Close() error {
    // release resources
    return nil
}
```

The contract:
- `Send` and `Receive` must be safe for concurrent use.
- `Receive` must unblock when `ctx` is cancelled and return `ctx.Err()`.
- `Send` must not modify the message.
- `Close` may be called multiple times; subsequent calls should be no-ops.
