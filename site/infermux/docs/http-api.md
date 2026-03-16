---
title: "HTTP API"
description: "OpenAI-compatible inference endpoints and infermux management endpoints. Request/response schemas and authentication."
---

# HTTP API

infermux exposes two sets of endpoints: the **inference API**, which is OpenAI-compatible, and the **management API**, which provides health, status, and cost information.

## Authentication

**Inference endpoints** (`/v1/*`): infermux can operate in two authentication modes.

- **Passthrough** (default): the `Authorization: Bearer <token>` header from the caller is forwarded to the provider. If your application already manages API keys, this requires no additional configuration.
- **Static key**: infermux validates incoming requests against a configured static key and uses its own provider credentials for outbound requests. This is the right model when you want to centralize API key management in infermux and not expose provider keys to callers.

```yaml
auth:
  mode: static_key
  key: "${INFERMUX_API_KEY}"   # callers must send this as Bearer token
```

**Management endpoints** (`/_infermux/*`): Protected by `INFERMUX_MANAGEMENT_TOKEN` if set. Otherwise unauthenticated. Always bind the management listener to localhost or an internal network when running in production.

---

## Inference API

### POST /v1/chat/completions

Create a chat completion. This is the primary inference endpoint.

**Request:**

```json
{
  "model": "gpt-4o-mini",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "What is the capital of France?"}
  ],
  "temperature": 0.7,
  "max_tokens": 256,
  "stream": false
}
```

All standard OpenAI chat completion parameters are accepted and forwarded to the selected provider. Parameters that a provider does not support are silently dropped (for example, `logprobs` when routing to Anthropic).

**Response:**

```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1741046400,
  "model": "gpt-4o-mini",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Paris."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 28,
    "completion_tokens": 3,
    "total_tokens": 31
  }
}
```

**Response headers (infermux-specific):**

| Header | Description |
|--------|-------------|
| `X-Infermux-Provider` | Name of the provider that served the request |
| `X-Infermux-Model` | Model name as understood by the provider |
| `X-Infermux-Strategy` | Routing strategy that selected the provider |
| `X-Infermux-Route-Group` | Route group that matched, if any |
| `X-Infermux-Latency-Ms` | Provider response time in milliseconds |
| `X-Infermux-Cost` | Estimated cost in USD |
| `X-Infermux-Prompt-Tokens` | Prompt token count |
| `X-Infermux-Completion-Tokens` | Completion token count |
| `X-Infermux-Model-Override` | Set when a model was substituted (e.g., budget downgrade) |

**Streaming:**

Set `"stream": true` to receive a server-sent events (SSE) stream in the standard OpenAI format. infermux streams the response from the provider with minimal buffering. The cost and latency headers are included in the final `200 OK` response headers.

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4o-mini", "messages": [{"role": "user", "content": "Count to five"}], "stream": true}'
```

### POST /v1/completions

Text completions (legacy). Forwarded to providers that support it.

```json
{
  "model": "gpt-3.5-turbo-instruct",
  "prompt": "The capital of France is",
  "max_tokens": 10
}
```

### POST /v1/embeddings

Generate embeddings. Routed to embedding-capable providers.

```json
{
  "model": "text-embedding-3-small",
  "input": "The quick brown fox"
}
```

**Response:**

```json
{
  "object": "list",
  "data": [
    {
      "object": "embedding",
      "index": 0,
      "embedding": [0.0023, -0.0142, ...]
    }
  ],
  "model": "text-embedding-3-small",
  "usage": {
    "prompt_tokens": 5,
    "total_tokens": 5
  }
}
```

---

## Management API

All management endpoints are prefixed with `/_infermux/`.

### GET /_infermux/health

Liveness check. Returns 200 if the server is running, regardless of provider health.

```json
{"status": "ok"}
```

### GET /_infermux/status

Overall status including provider health summary.

```json
{
  "version": "0.4.0",
  "uptime_seconds": 86400,
  "providers_healthy": 2,
  "providers_total": 3,
  "requests_total": 481200,
  "requests_last_minute": 847,
  "errors_last_minute": 12
}
```

### GET /_infermux/providers

List all providers with health, circuit state, and routing metadata.

```json
[
  {
    "name": "openai",
    "type": "openai",
    "healthy": true,
    "circuit": "closed",
    "error_rate": 0.014,
    "p95_latency_ms": 412,
    "requests_last_minute": 624,
    "models": ["gpt-4o", "gpt-4o-mini"],
    "rate_limit_headroom_rpm": 2876
  }
]
```

### POST /_infermux/providers/{name}/circuit/open

Force a circuit open (remove provider from routing).

### POST /_infermux/providers/{name}/circuit/close

Force a circuit closed (restore provider to routing, bypasses probe).

### POST /_infermux/providers/{name}/circuit/reset

Reset circuit state and clear error rate statistics.

### GET /_infermux/metrics

Prometheus-compatible metrics exposition. Compatible with any Prometheus scraper.

```
# HELP infermux_requests_total Total inference requests by provider and model
# TYPE infermux_requests_total counter
infermux_requests_total{provider="openai",model="gpt-4o-mini",status="ok"} 47821
infermux_requests_total{provider="anthropic",model="claude-haiku-3-5",status="ok"} 12043
infermux_requests_total{provider="openai",model="gpt-4o",status="error"} 14

# HELP infermux_latency_seconds Inference request latency
# TYPE infermux_latency_seconds histogram
infermux_latency_seconds_bucket{provider="openai",model="gpt-4o-mini",le="0.5"} 38400
infermux_latency_seconds_bucket{provider="openai",model="gpt-4o-mini",le="1.0"} 46200
infermux_latency_seconds_bucket{provider="openai",model="gpt-4o-mini",le="5.0"} 47800
infermux_latency_seconds_bucket{provider="openai",model="gpt-4o-mini",le="+Inf"} 47821
infermux_latency_seconds_sum{provider="openai",model="gpt-4o-mini"} 22841.4
infermux_latency_seconds_count{provider="openai",model="gpt-4o-mini"} 47821

# HELP infermux_cost_usd_total Total estimated cost in USD
# TYPE infermux_cost_usd_total counter
infermux_cost_usd_total{provider="openai",model="gpt-4o-mini"} 7.23
infermux_cost_usd_total{provider="anthropic",model="claude-haiku-3-5"} 9.61

# HELP infermux_circuit_state Circuit breaker state (0=closed, 1=half-open, 2=open)
# TYPE infermux_circuit_state gauge
infermux_circuit_state{provider="openai"} 0
infermux_circuit_state{provider="anthropic"} 2
```

### GET /_infermux/costs

Aggregated cost report. See [Cost Tracking](/infermux/docs/cost-tracking/) for the full schema and query parameters.

## Error responses

When infermux cannot route a request, it returns an OpenAI-format error response with an HTTP 4xx or 5xx status:

```json
{
  "error": {
    "message": "no healthy providers available for model gpt-4o",
    "type": "infermux_error",
    "code": "no_healthy_providers"
  }
}
```

| Code | HTTP Status | Meaning |
|------|-------------|---------|
| `no_healthy_providers` | 503 | All eligible providers have open circuits or failed health checks |
| `model_not_found` | 404 | No provider is configured to serve the requested model |
| `budget_exceeded` | 429 | Caller's monthly budget has been exhausted |
| `rate_limit` | 429 | All eligible providers are at their rate limit |
| `upstream_error` | 502 | The selected provider returned an unexpected error |
| `upstream_timeout` | 504 | The selected provider did not respond within the configured timeout |
