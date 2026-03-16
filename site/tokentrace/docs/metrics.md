---
title: Metrics
description: Every built-in metric derived from spans — cost, tokens, latency, error rate, and quality score. How metrics are computed from spans and how to define custom metrics.
---

# Metrics

tokentrace derives a set of built-in metrics from the spans it receives. Metrics are computed continuously by an in-process aggregator as spans arrive. They are available via the HTTP API at `/metrics` and in Prometheus format at `/metrics/prometheus`. All metrics support time-window queries (1h, 24h, 7d, or a custom duration).

## Built-in metrics

### Cost metrics

**`total_cost`** — Total spend in USD across all spans in the window. This is the sum of the `Cost` field on every recorded span.

**`cost_by_model`** — Total cost broken down by the `Model` field. Returns a map of model name to USD amount.

**`cost_by_caller`** — Total cost broken down by the `Caller` field on each span. Useful for attributing spend to specific services, users, or workflows.

**`cost_per_call`** — Average cost per span (total_cost / span_count). Useful for detecting prompt length creep — if cost_per_call increases while span_count stays flat, your prompts are getting longer.

**`cost_by_attribute`** — Cost grouped by a specific attribute key. For example, `cost_by_attribute?key=workflow` returns cost broken down by the `workflow` attribute on each span. Only attribute keys that appear on at least one span in the window are returned.

### Token metrics

**`prompt_tokens`** — Total prompt tokens consumed in the window. Sum of `PromptTokens` across all spans.

**`completion_tokens`** — Total completion tokens in the window. Sum of `CompTokens` across all spans.

**`total_tokens`** — Total tokens (prompt + completion) in the window.

**`tokens_by_model`** — Total tokens broken down by model. Returns a map of model name to `{prompt, completion, total}` counts.

**`prompt_token_p95`** — The 95th percentile of `PromptTokens` across spans in the window. Useful for detecting outlier inputs driving up cost.

### Latency metrics

**`latency_p50`** — Median latency in milliseconds across all spans in the window.

**`latency_p95`** — 95th percentile latency in milliseconds. This is the most useful alert target — spikes here affect a meaningful fraction of users.

**`latency_p99`** — 99th percentile latency in milliseconds.

**`latency_by_model`** — Latency percentiles broken down by model. Returns a map of model name to `{p50, p95, p99}` in milliseconds.

**`ttft_p50`**, **`ttft_p95`** — Time to first token percentiles (streaming calls only). Spans without `TTFTMs` set are excluded from these metrics.

### Error metrics

**`error_rate`** — Fraction of spans with `Status != StatusOK` in the window. Range: [0, 1]. A value of 0.02 means 2% of calls are failing or timing out.

**`error_count`** — Absolute count of error and timeout spans in the window.

**`timeout_rate`** — Fraction of spans with `Status == StatusTimeout` specifically.

### Quality metrics

**`quality_score`** — Average value of the `eval.score` attribute across spans in the window that have this attribute set. Spans without `eval.score` are excluded. Range: [0, 1].

**`quality_p10`** — 10th percentile of `eval.score`. Tracks your worst-case quality, which is often more actionable than average quality.

**`quality_by_model`** — Average `eval.score` broken down by model. Use this to compare model versions.

**`quality_by_attribute`** — Average `eval.score` grouped by a specific attribute key. For example, `quality_by_attribute?key=document_type` shows quality differences across document categories.

## How metrics are computed

The aggregator maintains two data structures per metric: a **sliding window ring buffer** for time-windowed queries and a **total counter** for all-time aggregates.

When a span arrives:
1. The span is appended to the ring buffer with a timestamp.
2. All counter metrics (total_cost, prompt_tokens, etc.) are incremented.
3. All histogram metrics (latency, ttft, prompt_token distribution) add the span's value to the histogram bucket.
4. Custom metric rules (if any) are evaluated against the span's attributes.

When a metric query arrives:
1. The window parameter (e.g., `?window=1h`) is parsed to a `time.Duration`.
2. The ring buffer is scanned for spans within the window.
3. The requested metric is computed from those spans.
4. The result is returned as JSON.

Percentile metrics (p50, p95, p99) use t-digest for approximate computation with bounded memory. For windows with fewer than 100 spans, exact percentiles are returned.

## Querying metrics

Via the HTTP API:

```
GET /metrics?window=1h
```

Returns all built-in metrics as a single JSON object.

```
GET /metrics/cost?window=24h
```

Returns only cost metrics for the last 24 hours.

```
GET /metrics/quality?window=7d&groupby=model
```

Returns quality metrics grouped by model over 7 days.

Window values: `1h`, `6h`, `24h`, `7d`, `30d`, or a Go duration string like `2h30m`.

Via Prometheus:

```
GET /metrics/prometheus
```

Returns all metrics in Prometheus text exposition format. See the [Metrics Reference](/tokentrace/docs/metrics-reference/) for the exact metric names and labels used in the Prometheus output.

## Custom metrics

Define a custom metric by providing a `MetricDef` in the tracer config:

```go
tracer := tokentrace.New(tokentrace.Config{
    Transport: tokentrace.FileTransport("./traces.jsonl"),
    CustomMetrics: []tokentrace.MetricDef{
        {
            Name:      "legal_doc_cost",
            Type:      tokentrace.MetricSum,
            Field:     tokentrace.FieldCost,
            FilterKey: "document_type",
            FilterVal: "legal",
        },
        {
            Name:  "agent_step_count",
            Type:  tokentrace.MetricCount,
            // Count spans where step attribute is set (any value)
            FilterKey: "step",
        },
    },
})
```

Custom metrics appear in the `/metrics` response alongside built-in metrics and are exported to Prometheus with the `tokentrace_custom_` prefix.

**Metric types:**

- `MetricSum` — sum of a numeric field across matching spans
- `MetricCount` — count of matching spans
- `MetricAvg` — average of a numeric field
- `MetricP95` — 95th percentile of a numeric field

**Available fields for aggregation:** `FieldCost`, `FieldPromptTokens`, `FieldCompTokens`, `FieldTotalTokens`, `FieldLatencyMs`, `FieldTTFTMs`, or any attribute key prefixed with `attr.` (e.g., `"attr.eval.score"`).

## Next steps

- [Metrics Reference](/tokentrace/docs/metrics-reference/) — Complete table of every built-in metric name, type, unit, and Prometheus labels.
- [Alerts](/tokentrace/docs/alerts/) — Alert rules that fire when metrics cross thresholds.
- [HTTP API](/tokentrace/docs/http-api/) — Full endpoint reference for querying metrics.
