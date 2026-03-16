---
title: protocol
description: The protocol package — Message envelope, type constants, source identifiers, structured payload types, versioning, and wire format.
---

# protocol

Import path: `github.com/greynewell/mist-go/protocol`

The `protocol` package defines the universal message envelope used for all MIST inter-tool communication. Every message that crosses a tool boundary — inference requests, eval results, trace spans, schema definitions, health pings — is wrapped in a `Message`. The envelope is transport-agnostic: the same struct is serialized to JSON and carried over HTTP, file, stdio, or in-process channels without modification.

## Message

`Message` is the envelope for all MIST communication:

```go
type Message struct {
    Version     string          `json:"version"`
    ID          string          `json:"id"`
    Source      string          `json:"source"`
    Type        string          `json:"type"`
    TimestampNS int64           `json:"timestamp_ns"`
    Payload     json.RawMessage `json:"payload"`
    Checksum    uint32          `json:"checksum,omitempty"`
}
```

**Fields:**

- `Version` — Protocol version. Currently `"1"`. Used by `CheckVersion` to detect incompatible messages.
- `ID` — Randomly generated 32-character lowercase hex string (128 bits of entropy from `crypto/rand`). Unique per message.
- `Source` — Identifier of the tool that created this message. Use the predefined `Source*` constants or a custom string.
- `Type` — Message type. Use the predefined `Type*` constants to identify the payload.
- `TimestampNS` — Unix nanosecond timestamp when the message was created (`time.Now().UnixNano()`).
- `Payload` — JSON-encoded message body. Use `Decode` to unmarshal into a typed struct.
- `Checksum` — Optional CRC32 IEEE checksum of the payload bytes. Zero means integrity checking is disabled for this message.

The maximum allowed serialized message size is 10 MB (`MaxMessageSize = 10 << 20`). `Unmarshal` returns an error if this limit is exceeded.

## Creating messages

Use `New` to create a message with a random ID and current timestamp:

```go
msg, err := protocol.New("matchspec", protocol.TypeEvalRun, protocol.EvalRun{
    Suite:    "swe-bench-verified",
    InferURL: "http://localhost:8081",
})
if err != nil {
    return err
}
```

`New` takes the source identifier, the type constant, and any value that can be JSON-marshaled as the payload. It returns `(*Message, error)` — the error is non-nil only if the payload cannot be marshaled.

## Reading message payloads

Use `Decode` to unmarshal the payload into a typed struct:

```go
var run protocol.EvalRun
if err := msg.Decode(&run); err != nil {
    return fmt.Errorf("invalid EvalRun payload: %w", err)
}
fmt.Println(run.Suite)
```

Dispatch on `msg.Type` to decide which struct to decode into:

```go
switch msg.Type {
case protocol.TypeInferRequest:
    var req protocol.InferRequest
    if err := msg.Decode(&req); err != nil {
        return err
    }
    return handleInferRequest(ctx, req)

case protocol.TypeHealthPing:
    var ping protocol.HealthPing
    if err := msg.Decode(&ping); err != nil {
        return err
    }
    return handlePing(ctx, ping)

default:
    return fmt.Errorf("unknown message type: %s", msg.Type)
}
```

## Serialization

```go
// Marshal to JSON bytes.
data, err := msg.Marshal()

// Marshal with CRC32 checksum included.
data, err := msg.MarshalWithChecksum()

// Unmarshal from JSON bytes (validates envelope, checks size limit).
msg, err := protocol.Unmarshal(data)

// Verify checksum (returns true if checksum is absent or valid).
if !msg.VerifyChecksum() {
    return fmt.Errorf("message integrity check failed: %s", msg.ID)
}
```

## Validation

`Validate` checks that required fields are present and the payload is within the size limit:

```go
if err := msg.Validate(); err != nil {
    log.Printf("dropping invalid message: %v", err)
}
```

`Unmarshal` calls `Validate` automatically. You only need to call `Validate` directly if you constructed a `Message` struct manually.

## Message type constants

```go
// Data pipeline (SchemaFlux)
TypeDataEntities = "data.entities" // batch of compiled entities
TypeDataSchema   = "data.schema"   // schema definition

// Inference (InferMux)
TypeInferRequest  = "infer.request"  // LLM inference request
TypeInferResponse = "infer.response" // LLM inference response

// Evaluation (MatchSpec)
TypeEvalRun    = "eval.run"    // start an evaluation
TypeEvalResult = "eval.result" // evaluation outcome

// Observability (TokenTrace)
TypeTraceSpan  = "trace.span"  // a single trace span
TypeTraceAlert = "trace.alert" // quality/cost/latency alert

// Health (all tools)
TypeHealthPing = "health.ping"
TypeHealthPong = "health.pong"
```

## Source identifier constants

```go
SourceSchemaFlux = "schemaflux"
SourceInferMux   = "infermux"
SourceMatchSpec  = "matchspec"
SourceTokenTrace = "tokentrace"
```

Use these when creating messages from within a MIST tool. For custom tools, use a descriptive lowercase string.

## Payload types

All structured payload types are defined in the `protocol` package.

### Inference

```go
type InferRequest struct {
    Model    string            // model name or "auto" for routing
    Provider string            // explicit provider or empty for auto
    Messages []ChatMessage     // conversation history
    Params   map[string]any    // temperature, max_tokens, etc.
    Meta     map[string]string // trace context, request tags
}

type ChatMessage struct {
    Role    string // "user", "assistant", "system", "tool"
    Content string
}

type InferResponse struct {
    Model        string
    Provider     string
    Content      string
    TokensIn     int64
    TokensOut    int64
    CostUSD      float64
    LatencyMS    int64
    FinishReason string
}
```

### Evaluation

```go
type EvalRun struct {
    Suite    string            // benchmark suite name
    Tasks    []string          // specific tasks, empty for all
    Baseline bool              // run without tools for baseline comparison
    InferURL string            // InferMux endpoint URL
    Tags     map[string]string // metadata tags
}

type EvalResult struct {
    Suite      string
    Task       string
    Passed     bool
    Score      float64
    Baseline   float64
    Delta      float64 // Score - Baseline
    DurationMS int64
    Error      string
}
```

### Observability

```go
type TraceSpan struct {
    TraceID   string
    SpanID    string
    ParentID  string
    Operation string
    StartNS   int64
    EndNS     int64
    Status    string         // "ok" or "error"
    Attrs     map[string]any // model, provider, tokens_in, tokens_out, cost_usd, etc.
}

type TraceAlert struct {
    Level     string  // "warning" or "critical"
    Metric    string  // "latency_p99", "cost_hourly", "error_rate"
    Value     float64
    Threshold float64
    Message   string
}
```

### Health

```go
type HealthPing struct {
    From string // sender's source identifier
}

type HealthPong struct {
    From    string
    Version string
    Uptime  int64 // seconds
}
```

### Data

```go
type DataEntities struct {
    Count    int
    Format   string // "json", "jsonl", "csv"
    Path     string // path to entity data file
    Schema   string // schema identifier
    Checksum string // optional integrity check
}

type DataSchema struct {
    Name   string
    Fields []SchemaField
}

type SchemaField struct {
    Name     string
    Type     string
    Required bool
}
```

## Versioning

The protocol version is `"1"`. `CheckVersion` validates a message version against the supported range:

```go
if err := protocol.CheckVersion(msg.Version); err != nil {
    return err // version too old or too new
}
```

When two tools connect, they can negotiate the highest mutually supported version:

```go
// local supports "1-2", remote reports "1"
agreed, err := protocol.NegotiateVersion("1-2", "1")
// agreed == "1", err == nil
```

`IsCompatible` is a boolean convenience wrapper:

```go
if !protocol.IsCompatible(msg.Version) {
    return fmt.Errorf("unsupported protocol version: %s", msg.Version)
}
```

## Wire format

Messages are serialized as a single JSON object. Example wire format:

```json
{
  "version": "1",
  "id": "a3f2b1c4d5e6f7a8b9c0d1e2f3a4b5c6",
  "source": "matchspec",
  "type": "eval.run",
  "timestamp_ns": 1710000000000000000,
  "payload": {"suite":"swe-bench-verified","baseline":false,"infer_url":"http://localhost:8081"}
}
```

For file and stdio transports, messages are written one per line (newline-delimited JSON). For HTTP transport, each message is the body of a single POST request to `/mist`.

The `Checksum` field is absent when zero (omitempty). When present, it is the CRC32 IEEE checksum of the raw `payload` bytes before JSON encoding of the envelope.
