---
title: Transports
description: tokentrace transports — FileTransport, HTTPTransport, StdoutTransport, and MultiTransport. Configuration, batching behavior, and retry on HTTP failure.
---

# Transports

A transport is responsible for delivering spans from your application to a sink. tokentrace ships four built-in transports and a `Transport` interface for writing your own. The transport is configured once when you create a `Tracer` and applies to all spans that tracer records.

## Transport interface

```go
type Transport interface {
    // Send delivers one or more spans to the sink.
    // Implementations must be safe for concurrent use.
    Send(ctx context.Context, spans []Span) error

    // Flush blocks until all buffered spans have been delivered.
    Flush(ctx context.Context) error

    // Close shuts down the transport and releases resources.
    Close() error
}
```

## StdoutTransport

Writes each span as a JSON object to stdout. Intended for development and debugging.

```go
transport := tokentrace.StdoutTransport()
```

Spans are written synchronously. There is no buffering, batching, or retry. Each span is followed by a newline. Output is human-readable but also valid JSONL if you want to pipe it to `jq`.

**Options:**

```go
transport := tokentrace.StdoutTransportWith(tokentrace.StdoutOptions{
    Pretty: true, // indent JSON output
})
```

## FileTransport

Writes spans as JSONL (one JSON object per line) to a file on disk. Each span is appended atomically. Suitable for development, local testing, and low-throughput production use.

```go
transport := tokentrace.FileTransport("./traces.jsonl")
```

**Options:**

```go
transport := tokentrace.FileTransportWith(tokentrace.FileOptions{
    Path:       "./traces.jsonl",
    Rotate:     true,           // rotate the file daily
    MaxSizeMB:  100,            // rotate when file exceeds 100 MB
    MaxFiles:   7,              // keep 7 rotated files
    Sync:       false,          // call fsync after each write (safer, slower)
    BufferSize: 256 * 1024,     // write buffer size in bytes (default 256 KB)
})
```

Writes are buffered and flushed on a 500 ms timer or when the buffer is full, whichever comes first. Call `transport.Flush(ctx)` to force an immediate flush — important before calling `os.Exit` or during graceful shutdown.

FileTransport is safe for concurrent use. Multiple goroutines can record spans simultaneously.

## HTTPTransport

POSTs spans as JSON to an HTTP endpoint. Designed for production use — sends spans to a central collector, another tokentrace instance, or any HTTP endpoint that accepts the span payload.

```go
transport := tokentrace.HTTPTransport("https://collector.example.com/spans")
```

**Options:**

```go
transport := tokentrace.HTTPTransportWith(tokentrace.HTTPOptions{
    Endpoint:      "https://collector.example.com/spans",
    BatchSize:     100,              // send up to 100 spans per request
    FlushInterval: 2 * time.Second,  // flush at least every 2 seconds
    Timeout:       10 * time.Second, // HTTP request timeout
    MaxRetries:    3,                // retry failed requests up to 3 times
    RetryBackoff:  500 * time.Millisecond, // initial backoff, doubles each retry
    Headers: map[string]string{
        "Authorization": "Bearer " + os.Getenv("TOKENTRACE_TOKEN"),
    },
    Compression:   tokentrace.GzipCompression, // compress request bodies
    MaxQueueSize:  10000, // drop spans if queue exceeds this size
})
```

**Batching behavior:**

Spans are accumulated in an in-memory queue. A background goroutine flushes the queue either when it reaches `BatchSize` or when `FlushInterval` elapses, whichever comes first. This means a low-traffic service with `FlushInterval: 2s` will send at most one request per 2 seconds, while a high-traffic service will send batches of 100 as fast as they fill.

**Retry on failure:**

If the HTTP POST returns a 5xx status or a network error, the batch is retried up to `MaxRetries` times with exponential backoff starting at `RetryBackoff`. 4xx responses are not retried — they indicate a problem with the request itself (bad auth, malformed payload) that will not resolve on retry. Failed batches that exhaust all retries are logged and dropped.

**Backpressure:**

If the queue exceeds `MaxQueueSize`, new spans are dropped (not blocked). This prevents the transport from consuming unbounded memory under sustained write pressure. Dropped span counts are tracked in the metric `tokentrace_transport_dropped_total`.

## MultiTransport

Fans out each span to multiple transports in parallel. All transports receive every span.

```go
transport := tokentrace.MultiTransport(
    tokentrace.FileTransport("./traces.jsonl"),
    tokentrace.HTTPTransport("https://collector.example.com/spans"),
)
```

`Send` returns an error if any child transport returns an error. `Flush` and `Close` are called on all child transports. A failure in one transport does not prevent delivery to others.

Typical use case: write to a local file for debugging while also sending to a central collector in production.

## NoopTransport

Discards all spans. Useful in tests when you want to exercise instrumented code without any I/O side effects.

```go
transport := tokentrace.NoopTransport()
```

## Writing a custom transport

Implement the `Transport` interface. The example below writes spans to a Redis list:

```go
type RedisTransport struct {
    client *redis.Client
    key    string
}

func (t *RedisTransport) Send(ctx context.Context, spans []tokentrace.Span) error {
    for _, s := range spans {
        b, err := json.Marshal(s)
        if err != nil {
            return err
        }
        if err := t.client.RPush(ctx, t.key, b).Err(); err != nil {
            return err
        }
    }
    return nil
}

func (t *RedisTransport) Flush(ctx context.Context) error { return nil }
func (t *RedisTransport) Close() error                    { return nil }
```

Pass the custom transport to `tokentrace.New`:

```go
tracer := tokentrace.New(tokentrace.Config{
    Transport: &RedisTransport{client: redisClient, key: "tt:spans"},
})
```

## Choosing a transport

| Environment        | Recommended transport                      |
|--------------------|--------------------------------------------|
| Local development  | StdoutTransport or FileTransport           |
| Integration tests  | NoopTransport or a BufferTransport (see Go API) |
| Single-process app | FileTransport with rotation                |
| Distributed system | HTTPTransport to a central tokentrace node |
| Multiple sinks     | MultiTransport                             |

## Next steps

- [Configuration](/tokentrace/docs/config/) — Set the default transport in `tokentrace.yml`.
- [Go API](/tokentrace/docs/go-api/) — BufferTransport for testing. Async vs sync flush modes.
- [HTTP API](/tokentrace/docs/http-api/) — The span ingestion endpoint that HTTPTransport posts to.
