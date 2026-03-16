---
title: Metrics Reference
description: Complete reference table for every tokentrace built-in metric — name, type, unit, description, and Prometheus labels.
---

# Metrics Reference

This page lists every built-in metric that tokentrace derives from spans. All metrics are available via the [HTTP API](/tokentrace/docs/http-api/) and in Prometheus format at `/metrics/prometheus`.

Prometheus metric names use the prefix `tokentrace_`. The JSON API uses snake_case without the prefix.

## Cost metrics

| JSON name | Prometheus name | Type | Unit | Description |
|---|---|---|---|---|
| `total_cost` | `tokentrace_cost_total` | Counter | USD | Total spend across all spans in the window |
| `cost_per_call` | `tokentrace_cost_per_call` | Gauge | USD | Average cost per span in the window |
| `cost_by_model` | `tokentrace_cost_total` | Counter | USD | Total cost, labelled by `model` |
| `cost_by_caller` | `tokentrace_cost_total` | Counter | USD | Total cost, labelled by `caller` |
| `cost_by_provider` | `tokentrace_cost_total` | Counter | USD | Total cost, labelled by `provider` |

**Prometheus labels for `tokentrace_cost_total`:** `model`, `provider`, `caller`. All label values default to `"unknown"` when the corresponding span field is empty.

## Token metrics

| JSON name | Prometheus name | Type | Unit | Description |
|---|---|---|---|---|
| `prompt_tokens` | `tokentrace_prompt_tokens_total` | Counter | tokens | Total prompt tokens in the window |
| `completion_tokens` | `tokentrace_completion_tokens_total` | Counter | tokens | Total completion tokens in the window |
| `total_tokens` | `tokentrace_tokens_total` | Counter | tokens | Total tokens (prompt + completion) in the window |
| `prompt_token_p50` | `tokentrace_prompt_tokens` | Histogram | tokens | 50th percentile prompt tokens per span |
| `prompt_token_p95` | `tokentrace_prompt_tokens` | Histogram | tokens | 95th percentile prompt tokens per span |
| `completion_token_p50` | `tokentrace_completion_tokens` | Histogram | tokens | 50th percentile completion tokens per span |
| `completion_token_p95` | `tokentrace_completion_tokens` | Histogram | tokens | 95th percentile completion tokens per span |
| `tokens_by_model` | `tokentrace_tokens_total` | Counter | tokens | Total tokens, labelled by `model` and `token_type` (`prompt`/`completion`) |

**Prometheus histogram buckets for token counts:** 64, 128, 256, 512, 1024, 2048, 4096, 8192, 16384, 32768.

## Latency metrics

| JSON name | Prometheus name | Type | Unit | Description |
|---|---|---|---|---|
| `latency_p50` | `tokentrace_latency_ms` | Histogram | ms | 50th percentile latency across all spans |
| `latency_p95` | `tokentrace_latency_ms` | Histogram | ms | 95th percentile latency across all spans |
| `latency_p99` | `tokentrace_latency_ms` | Histogram | ms | 99th percentile latency across all spans |
| `latency_by_model` | `tokentrace_latency_ms` | Histogram | ms | Latency percentiles, labelled by `model` |
| `ttft_p50` | `tokentrace_ttft_ms` | Histogram | ms | 50th percentile time to first token (streaming) |
| `ttft_p95` | `tokentrace_ttft_ms` | Histogram | ms | 95th percentile time to first token (streaming) |

**Prometheus histogram buckets for latency:** 50, 100, 200, 500, 1000, 2000, 3000, 5000, 10000, 30000 (milliseconds).

**Note:** Spans with `TTFTMs == 0` are excluded from TTFT metrics. Spans with `LatencyMs == 0` are excluded from latency metrics. A span with zero latency is treated as unset, not as a genuine sub-millisecond call.

## Error metrics

| JSON name | Prometheus name | Type | Unit | Description |
|---|---|---|---|---|
| `error_rate` | `tokentrace_error_rate` | Gauge | fraction | Fraction of spans with status error or timeout |
| `error_count` | `tokentrace_errors_total` | Counter | spans | Count of spans with status error or timeout |
| `timeout_rate` | `tokentrace_timeout_rate` | Gauge | fraction | Fraction of spans with status timeout specifically |
| `timeout_count` | `tokentrace_timeouts_total` | Counter | spans | Count of spans with status timeout |
| `span_count` | `tokentrace_spans_total` | Counter | spans | Total spans recorded, labelled by `status` |

**Labels for `tokentrace_spans_total`:** `status` (`ok`, `error`, `timeout`), `model`, `provider`.

## Quality metrics

| JSON name | Prometheus name | Type | Unit | Description |
|---|---|---|---|---|
| `quality_score` | `tokentrace_quality_score` | Gauge | [0,1] | Average eval.score across scored spans in the window |
| `quality_p10` | `tokentrace_quality_score_p10` | Gauge | [0,1] | 10th percentile eval.score |
| `quality_p50` | `tokentrace_quality_score_p50` | Gauge | [0,1] | 50th percentile eval.score |
| `quality_by_model` | `tokentrace_quality_score` | Gauge | [0,1] | Average eval.score, labelled by `model` |

**Note:** Quality metrics are only computed for spans that include an `eval.score` attribute. Spans without this attribute are excluded entirely — they do not count as zero-scored spans.

## Transport metrics

These metrics describe the health of the tokentrace transport layer itself.

| JSON name | Prometheus name | Type | Unit | Description |
|---|---|---|---|---|
| `transport_sent` | `tokentrace_transport_sent_total` | Counter | spans | Spans successfully delivered to the sink |
| `transport_failed` | `tokentrace_transport_failed_total` | Counter | spans | Spans that failed delivery after all retries |
| `transport_dropped` | `tokentrace_transport_dropped_total` | Counter | spans | Spans dropped due to queue overflow |
| `transport_queue_depth` | `tokentrace_transport_queue_depth` | Gauge | spans | Current number of spans waiting to be flushed |

## Alert metrics

| JSON name | Prometheus name | Type | Unit | Description |
|---|---|---|---|---|
| `alert_firings` | `tokentrace_alert_firings_total` | Counter | firings | Total alert rule firings, labelled by `rule` |
| `alert_delivery_failed` | `tokentrace_alert_delivery_failed_total` | Counter | firings | Alert deliveries that failed, labelled by `rule` |

## Custom metrics

Custom metrics defined via `MetricDef` are exported to Prometheus with the prefix `tokentrace_custom_` and the metric name you provide. The type and unit are derived from the `MetricDef.Type` field.

Example: a custom metric named `legal_doc_cost` with type `MetricSum` appears as `tokentrace_custom_legal_doc_cost_total` in Prometheus output.

## Next steps

- [Metrics](/tokentrace/docs/metrics/) — How metrics are computed and how to define custom metrics.
- [HTTP API](/tokentrace/docs/http-api/) — Query metric values by name and window.
- [Grafana Integration](/tokentrace/docs/grafana/) — Use these metric names to build Grafana panels.
