---
title: Overview
description: The LLM observability gap, what tokentrace traces, system architecture, and how it fits into the MIST stack.
---

# Overview

tokentrace is a token-level observability library for AI inference systems. It gives you structured visibility into every inference call: what model ran, how many tokens it consumed, what it cost, how long it took, and — when combined with eval scores from [matchspec](/matchspec/) — how good the output was. Metrics are aggregated continuously and exposed via an HTTP API and Prometheus endpoint. Alert rules fire when cost, latency, or quality moves outside acceptable bounds.

## The LLM observability gap

Standard application performance monitoring tools track three signals: availability (is the endpoint up?), latency (how long does it take?), and error rate (what fraction of requests return non-2xx?). These signals were designed for deterministic systems where an HTTP 200 means the function ran correctly.

Language models break this assumption. A model call can return HTTP 200 in 400 ms with a hallucinated citation, a misunderstood instruction, or a response that confidently answers the wrong question. None of that is visible in your existing dashboards. Your error rate is 0%. Your latency looks fine. Your users are getting bad answers.

The gap is structural, not a tooling shortcoming. Traditional APM has no concept of output quality because traditional APIs produce deterministic outputs — a `/users/{id}` endpoint either returns the right user record or it doesn't, and you can verify that in a test. LLMs produce probabilistic outputs. What "correct" means depends on context, and correctness can degrade silently as models are updated, prompts drift, or input distributions shift.

tokentrace addresses this in three ways:

1. **Structured token traces** — Every inference call emits a span with token counts, cost, and latency as first-class fields, not buried in log strings. This makes cost and latency directly queryable.

2. **Custom attributes** — Spans accept arbitrary key/value metadata. Attach an eval score from matchspec, a session ID, a workflow name, or a content category. Any attribute can be aggregated into a metric or used in an alert rule.

3. **Quality metrics** — By attaching eval scores to spans, quality becomes a time-series metric like any other. You can track quality over time, correlate it with model version changes, and alert on drops.

## What tokentrace traces

A `Span` represents a single inference call. Every span records:

- **Model** — the model identifier (e.g., `gpt-4o`, `claude-3-5-sonnet-20241022`)
- **Provider** — the API provider (e.g., `openai`, `anthropic`, `bedrock`)
- **Prompt tokens** — tokens in the input context
- **Completion tokens** — tokens in the model output
- **Total tokens** — sum (some providers report this separately)
- **Cost** — computed cost in USD using tokentrace's built-in pricing table or a custom rate you provide
- **Latency** — wall-clock elapsed time in milliseconds from request dispatch to first byte of response
- **Time to first token** — time in milliseconds until streaming begins, when using streaming APIs
- **Status** — `ok`, `error`, or `timeout`
- **Error** — error string if status is not `ok`
- **Attributes** — map of string key to string or numeric value for custom metadata
- **Trace ID** — UUID assigned at `tokentrace.Start()` for correlating spans in a multi-step workflow
- **Span ID** — UUID for this individual span
- **Parent span ID** — for nesting spans in agent loops or pipelines
- **Caller** — optional identifier for the calling service or function

## Architecture

tokentrace has three layers:

**Span → Transport → Sink**

A `Span` is created when you call `trace.Record()`. The `Tracer` passes it to the configured `Transport`, which serializes it and delivers it to a sink. The sink might be a local file, an HTTP endpoint, stdout, or a combination of all three via `MultiTransport`.

Separately, a metrics aggregator consumes spans in the background. It maintains running counters and histograms for all built-in metrics (cost, tokens, latency, quality score, error rate). The HTTP API and Prometheus endpoint read from this aggregator.

Alert rules run on a timer against the metrics aggregator. When a rule fires, the configured delivery channel (HTTP webhook or stdout) receives an alert payload.

```
Your Go code
     │
     ▼
 tokentrace.Start()
     │
     ▼
  Span{...}   ──►  Transport  ──►  File / HTTP / Stdout
     │
     ▼
 Aggregator  ──►  Metrics API  ──►  Prometheus / Grafana
     │
     ▼
 Alert engine  ──►  Webhook / Stdout
```

The aggregator is in-process. There is no background daemon, no sidecar, and no separate collector to deploy. For high-throughput production systems, the HTTP transport batches spans and delivers them asynchronously; the metrics aggregation and alert evaluation happen on the receiving end, not in the application process.

## Relationship to mist-go

tokentrace is built on [mist-go](/mist-go/), the shared core library for the MIST stack. It uses:

- **mist-go/trace** — trace context model, span ID generation, correlation ID propagation
- **mist-go/metrics** — counter, gauge, and histogram primitives used by the aggregator
- **mist-go/transport** — the `Transport` interface, retry logic, batching, and HTTP delivery

You do not need to interact with mist-go directly to use tokentrace. The zero-dependency guarantee holds: the tokentrace binary has no runtime dependencies beyond the Go standard library.

## Use cases

**Cost visibility** — Know exactly what your AI features cost per call, per user, and per model. Set budget alerts before you exceed a spending threshold.

**Latency monitoring** — Track p50, p95, and p99 latency for each model and workflow. Detect regressions when providers degrade or when input length trends upward.

**Quality tracking** — Attach matchspec eval scores to spans. Track quality as a time-series metric. Alert on drops before users file support tickets.

**Model comparison** — Run two models behind a feature flag, attribute spans to each variant, and compare cost, latency, and quality directly in Grafana.

**Agent debugging** — Instrument each step of a multi-step agent loop with a child span. Trace the full token cost of a complex workflow. Identify which step is the latency bottleneck.

**Budget enforcement** — Set a hard cost limit per workflow. tokentrace's `Budget` helper accumulates span costs and returns an error when the limit is reached, stopping the agent loop before it overspends.

## Next steps

- [Quick Start](/tokentrace/docs/quick-start/) — Instrument your first inference call in five minutes.
- [Traces & Spans](/tokentrace/docs/traces/) — Complete reference for the Span struct and trace context model.
- [Alerts](/tokentrace/docs/alerts/) — Define cost, latency, and quality alert rules.
