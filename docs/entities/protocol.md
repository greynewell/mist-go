---
title: Protocol
slug: protocol
order: 2
---

# MIST Protocol

Every message between MIST tools uses a universal JSON envelope.

## Message Envelope

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

## Message Types

| Type | Source | Purpose |
|------|--------|---------|
| `data.entities` | SchemaFlux | Batch of compiled entities |
| `data.schema` | SchemaFlux | Schema definition |
| `infer.request` | MatchSpec | LLM inference request |
| `infer.response` | InferMux | LLM inference response |
| `eval.run` | Orchestrator | Start evaluation |
| `eval.result` | MatchSpec | Evaluation outcome |
| `trace.span` | Any | Trace span (MTTP) |
| `trace.alert` | TokenTrace | Quality/cost/latency alert |
| `health.ping` | Any | Liveness check |
| `health.pong` | Any | Liveness response |

## Creating Messages

```go
msg, err := protocol.New(protocol.SourceMatchSpec, protocol.TypeEvalResult,
    protocol.EvalResult{
        Suite:  "math",
        Task:   "addition",
        Passed: true,
        Score:  0.95,
    })
```

## Decoding Payloads

```go
var result protocol.EvalResult
if err := msg.Decode(&result); err != nil {
    log.Fatal(err)
}
```
