---
title: HTTP API
description: Complete reference for every tokentrace HTTP endpoint — span ingestion, metrics queries, trace retrieval, alert management, and health check.
---

# HTTP API

tokentrace exposes an HTTP API for ingesting spans, querying metrics, retrieving traces, and managing alert rules. Enable the server by setting `HTTPServer` in your tracer config or `http_server` in `tokentrace.yml`.

All endpoints return JSON. Error responses use a standard envelope:

```json
{"error": "message describing what went wrong"}
```

Default base URL: `http://localhost:9090`.

---

## POST /spans

Ingest one or more spans. This is the endpoint that `HTTPTransport` posts to. You can also call it directly to ingest spans from non-Go services or from tests.

**Request body:**

```json
[
  {
    "name":          "summarize-document",
    "model":         "gpt-4o",
    "provider":      "openai",
    "prompt_tokens": 512,
    "comp_tokens":   128,
    "total_tokens":  640,
    "cost_usd":      0.00448,
    "latency_ms":    1240,
    "status":        "ok",
    "caller":        "summarizer-service",
    "attributes": {
      "eval.score":    0.87,
      "workflow":      "document-summary"
    }
  }
]
```

You may POST a single span (as a JSON object) or an array of spans.

**Response:**

```
HTTP 202 Accepted
{"accepted": 1}
```

**Errors:**

- `400 Bad Request` — malformed JSON or missing required fields (`model`, at least one token count)
- `413 Request Entity Too Large` — batch exceeds the configured `max_batch_size` (default: 1000 spans)

---

## GET /metrics

Returns all built-in metrics for the specified time window.

**Query parameters:**

| Parameter | Type | Default | Description |
|---|---|---|---|
| `window` | duration | `1h` | Time window: `1h`, `6h`, `24h`, `7d`, `30d`, or a Go duration string |
| `groupby` | string | — | Group a metric by an attribute key or built-in dimension (`model`, `caller`, `provider`) |

**Response:**

```json
{
  "window": "1h",
  "span_count": 312,
  "cost": {
    "total_usd": 1.42,
    "per_call_usd": 0.00456,
    "by_model": {"gpt-4o": 1.28, "gpt-4o-mini": 0.14}
  },
  "tokens": {
    "prompt_total": 156800,
    "completion_total": 39200,
    "total": 196000
  },
  "latency": {
    "p50_ms": 820,
    "p95_ms": 2140,
    "p99_ms": 3810
  },
  "errors": {
    "rate": 0.013,
    "count": 4
  },
  "quality": {
    "score_avg": 0.884,
    "score_p10": 0.71,
    "scored_span_count": 180
  }
}
```

---

## GET /metrics/{name}

Returns a single metric by name.

**Path parameters:**

- `name` — metric name from the [Metrics Reference](/tokentrace/docs/metrics-reference/). Examples: `cost`, `latency`, `quality`, `error_rate`.

**Query parameters:** same as `GET /metrics`.

**Example:**

```
GET /metrics/cost?window=24h
```

```json
{
  "window": "24h",
  "total_usd": 8.74,
  "per_call_usd": 0.00421,
  "by_model": {
    "gpt-4o":      7.91,
    "gpt-4o-mini": 0.83
  },
  "span_count": 2076
}
```

```
GET /metrics/latency?window=1h&groupby=model
```

```json
{
  "window": "1h",
  "by_model": {
    "gpt-4o":      {"p50_ms": 980,  "p95_ms": 2340, "p99_ms": 4100},
    "gpt-4o-mini": {"p50_ms": 320,  "p95_ms": 710,  "p99_ms": 1200}
  }
}
```

---

## GET /metrics/prometheus

Returns all metrics in Prometheus text exposition format. Suitable for scraping by a Prometheus server.

**Response:** `Content-Type: text/plain; version=0.0.4`

```
# HELP tokentrace_cost_total Total cost in USD.
# TYPE tokentrace_cost_total counter
tokentrace_cost_total{model="gpt-4o",provider="openai",caller="summarizer-service"} 7.91

# HELP tokentrace_latency_ms Inference latency in milliseconds.
# TYPE tokentrace_latency_ms histogram
tokentrace_latency_ms_bucket{le="100"} 12
tokentrace_latency_ms_bucket{le="500"} 148
tokentrace_latency_ms_bucket{le="1000"} 389
...
```

---

## GET /traces/{id}

Retrieves all spans for a specific trace ID.

**Path parameters:**

- `id` — trace ID returned by `trace.ID()` or present in any span's `trace_id` field.

**Response:**

```json
{
  "trace_id": "7f3a1c2b-e4d5-4f6a-8b9c-0d1e2f3a4b5c",
  "name": "summarize-document",
  "started_at": "2026-03-15T14:23:01.441Z",
  "ended_at": "2026-03-15T14:23:02.681Z",
  "total_cost_usd": 0.00448,
  "total_tokens": 640,
  "span_count": 1,
  "spans": [
    {
      "span_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
      "parent_span_id": null,
      "name": "summarize-document",
      "model": "gpt-4o",
      "provider": "openai",
      "prompt_tokens": 512,
      "comp_tokens": 128,
      "total_tokens": 640,
      "cost_usd": 0.00448,
      "latency_ms": 1240,
      "status": "ok",
      "started_at": "2026-03-15T14:23:01.441Z",
      "ended_at": "2026-03-15T14:23:02.681Z",
      "attributes": {
        "eval.score": 0.87
      }
    }
  ]
}
```

**Errors:**

- `404 Not Found` — trace ID not found (not recorded, or outside the retention window)

---

## GET /traces

Query spans by attribute value. Returns matching traces in reverse chronological order.

**Query parameters:**

| Parameter | Type | Description |
|---|---|---|
| `attr.{key}` | string | Filter to spans with this attribute key/value pair |
| `model` | string | Filter to spans with this model |
| `caller` | string | Filter to spans with this caller |
| `status` | string | Filter by status: `ok`, `error`, `timeout` |
| `since` | RFC3339 | Return traces started after this time |
| `until` | RFC3339 | Return traces started before this time |
| `limit` | int | Maximum number of traces to return (default: 20, max: 200) |
| `offset` | int | Offset for pagination |

**Example:**

```
GET /traces?attr.request_id=abc123
GET /traces?model=gpt-4o&status=error&limit=50
```

---

## GET /alerts

Returns all configured alert rules and their current state.

**Response:**

```json
[
  {
    "name":          "hourly-cost-spike",
    "metric":        "total_cost",
    "op":            "gt",
    "threshold":     10.00,
    "window":        "1h",
    "last_evaluated": "2026-03-15T15:00:00.000Z",
    "last_fired":    "2026-03-15T14:00:00.000Z",
    "current_value": 1.42,
    "firing":        false,
    "silenced":      false,
    "silenced_until": null
  }
]
```

---

## POST /alerts

Create a new alert rule at runtime without restarting the process.

**Request body:**

```json
{
  "name":      "model-latency-alert",
  "metric":    "latency_p95",
  "op":        "gt",
  "threshold": 4000,
  "window":    "30m",
  "cooldown":  "1h",
  "min_spans": 10,
  "filter":    {"model": "gpt-4o"},
  "delivery": {
    "type": "http",
    "url":  "https://hooks.example.com/alerts"
  }
}
```

**Response:**

```
HTTP 201 Created
{"rule_id": "alert_a1b2c3d4", "name": "model-latency-alert"}
```

---

## POST /alerts/{name}/silence

Silence an alert rule for a duration.

**Request body:**

```json
{"duration": "2h"}
```

**Response:**

```
HTTP 200 OK
{"silenced_until": "2026-03-15T17:00:00.000Z"}
```

---

## DELETE /alerts/{name}

Remove an alert rule. Only applies to rules created via the API; rules defined in code or config are restored on restart.

**Response:** `HTTP 204 No Content`

---

## GET /health

Health check endpoint. Returns `200 OK` when the server is running and the metrics aggregator is operational.

**Response:**

```json
{
  "status":       "ok",
  "version":      "0.4.0",
  "uptime_s":     3612,
  "span_count":   8941,
  "transport":    "file",
  "queue_depth":  0
}
```

Returns `503 Service Unavailable` if the aggregator is unhealthy (e.g., disk full when using FileTransport).

---

## Authentication

The HTTP server does not enforce authentication by default. To require a bearer token:

```yaml
# tokentrace.yml
http_server:
  addr: ":9090"
  auth_token: "${TOKENTRACE_API_TOKEN}"
```

All requests must include `Authorization: Bearer <token>`. Requests without a valid token receive `401 Unauthorized`.

## Next steps

- [Configuration](/tokentrace/docs/config/) — Enable and configure the HTTP server.
- [Alerts](/tokentrace/docs/alerts/) — Alert rule structure and delivery options.
- [Metrics Reference](/tokentrace/docs/metrics-reference/) — Every metric name available at `/metrics/{name}`.
