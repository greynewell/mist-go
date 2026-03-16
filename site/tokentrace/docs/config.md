---
title: Configuration
description: tokentrace.yml schema — transport config, alert rules, retention settings, and HTTP server configuration.
---

# Configuration

tokentrace can be configured in code via `tokentrace.Config` or via a `tokentrace.yml` file. When both are present, the YAML file takes precedence for fields it explicitly sets. Environment variable substitution is supported using the `${VAR}` syntax.

## Loading configuration

By default, tokentrace looks for `tokentrace.yml` in the current working directory. You can specify a path explicitly:

```go
tracer, err := tokentrace.NewFromFile("./config/tokentrace.yml")
```

Or load from an `io.Reader`:

```go
f, _ := os.Open("tokentrace.yml")
tracer, err := tokentrace.NewFromReader(f)
```

## Full schema

```yaml
# tokentrace.yml

# transport — where spans are delivered
transport:
  type: file          # "file", "http", "stdout", "multi", or "noop"

  # For type: file
  file:
    path: ./traces.jsonl
    rotate: true
    max_size_mb: 100
    max_files: 7
    sync: false
    buffer_size_kb: 256

  # For type: http
  http:
    endpoint: https://collector.example.com/spans
    batch_size: 100
    flush_interval: 2s
    timeout: 10s
    max_retries: 3
    retry_backoff: 500ms
    compression: gzip       # "gzip" or "none"
    max_queue_size: 10000
    headers:
      Authorization: "Bearer ${TOKENTRACE_TOKEN}"

  # For type: multi — fan out to multiple transports
  multi:
    - type: file
      file:
        path: ./traces.jsonl
    - type: http
      http:
        endpoint: https://collector.example.com/spans

# http_server — expose metrics and ingestion API
http_server:
  enabled: true
  addr: ":9090"
  metrics_path: /metrics
  prometheus_path: /metrics/prometheus
  ingest_path: /spans
  health_path: /health
  read_timeout: 5s
  write_timeout: 10s
  auth_token: "${TOKENTRACE_API_TOKEN}"   # optional; omit to disable auth

# retention — how long spans are kept in the in-process ring buffer
# and (if using FileTransport with the HTTP server) on disk
retention:
  memory_window: 7d    # keep up to 7 days of spans in memory
  disk_days: 30        # keep rotated JSONL files for 30 days (FileTransport only)

# alerts — list of alert rules
alerts:
  - name: hourly-cost-spike
    metric: total_cost
    op: gt
    threshold: 10.00
    window: 1h
    cooldown: 4h
    min_spans: 5
    delivery:
      type: http
      url: https://hooks.example.com/alerts
      timeout: 5s
      headers:
        Authorization: "Bearer ${ALERT_TOKEN}"

  - name: p95-latency-regression
    metric: latency_p95
    op: gt
    threshold: 3000
    window: 30m
    delivery:
      type: stdout

  - name: quality-drop
    metric: quality_score
    op: lt
    threshold: 0.75
    window: 1h
    min_spans: 20
    filter:
      model: gpt-4o
    delivery:
      type: http
      url: https://hooks.example.com/alerts

  - name: error-rate-spike
    metric: error_rate
    op: gt
    threshold: 0.05
    window: 15m
    min_spans: 10
    delivery:
      type: stdout

# custom_metrics — derived metrics beyond the built-ins
custom_metrics:
  - name: legal_doc_cost
    type: sum
    field: cost
    filter_key: document_type
    filter_val: legal

  - name: agent_step_count
    type: count
    filter_key: step

# pricing — override built-in model pricing
pricing:
  models:
    my-fine-tuned-model:
      prompt_per_1m: 3.00
      completion_per_1m: 12.00
```

## Field reference

### transport

| Field | Type | Default | Description |
|---|---|---|---|
| `type` | string | `"stdout"` | Transport type: `file`, `http`, `stdout`, `multi`, `noop` |

### transport.file

| Field | Type | Default | Description |
|---|---|---|---|
| `path` | string | `./traces.jsonl` | Path to the output file |
| `rotate` | bool | `false` | Enable log rotation |
| `max_size_mb` | int | `100` | Rotate when file exceeds this size |
| `max_files` | int | `7` | Number of rotated files to keep |
| `sync` | bool | `false` | Call fsync after each write |
| `buffer_size_kb` | int | `256` | Write buffer size in KB |

### transport.http

| Field | Type | Default | Description |
|---|---|---|---|
| `endpoint` | string | — | Required. URL to POST spans to |
| `batch_size` | int | `100` | Maximum spans per request |
| `flush_interval` | duration | `2s` | Maximum time between flushes |
| `timeout` | duration | `10s` | HTTP request timeout |
| `max_retries` | int | `3` | Retry attempts after failure |
| `retry_backoff` | duration | `500ms` | Initial retry backoff (doubles each attempt) |
| `compression` | string | `"none"` | `"gzip"` or `"none"` |
| `max_queue_size` | int | `10000` | Drop spans when queue exceeds this |
| `headers` | map | — | Additional HTTP headers |

### http_server

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | bool | `false` | Start the HTTP server |
| `addr` | string | `":9090"` | Listen address |
| `auth_token` | string | — | Bearer token required for all requests |
| `read_timeout` | duration | `5s` | HTTP read timeout |
| `write_timeout` | duration | `10s` | HTTP write timeout |

### alerts

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | yes | Unique name for this rule |
| `metric` | string | yes | Metric to evaluate |
| `op` | string | yes | `gt`, `gte`, `lt`, `lte` |
| `threshold` | float | yes | Threshold value |
| `window` | duration | yes | Time window for metric evaluation |
| `cooldown` | duration | no | Minimum time between firings (default: `window`) |
| `min_spans` | int | no | Minimum spans required to evaluate (default: 0) |
| `filter` | map | no | Key/value attribute filter |
| `delivery.type` | string | yes | `http` or `stdout` |
| `delivery.url` | string | if http | Webhook URL |
| `delivery.timeout` | duration | no | Delivery request timeout (default: `5s`) |
| `delivery.headers` | map | no | Additional HTTP headers for delivery |

## Environment variable substitution

Use `${VAR_NAME}` anywhere in the YAML file to substitute an environment variable at load time. If the variable is not set, the field is treated as an empty string. To require a variable to be set, use `${VAR_NAME:?error message}` — tokentrace will refuse to start and print the error message if the variable is unset.

```yaml
http:
  endpoint: "${COLLECTOR_URL:?COLLECTOR_URL must be set}"
  headers:
    Authorization: "Bearer ${TOKENTRACE_TOKEN:?TOKENTRACE_TOKEN must be set}"
```

## Next steps

- [Transports](/tokentrace/docs/transports/) — Transport implementation details and options.
- [Alerts](/tokentrace/docs/alerts/) — Alert rule semantics and delivery options.
- [HTTP API](/tokentrace/docs/http-api/) — Enable and use the HTTP server.
