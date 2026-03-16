---
title: "Cost Tracking"
description: "Per-request token accounting, pricing tables, cost attribution, budget enforcement, and cost reports."
---

# Cost Tracking

infermux records the cost of every inference request. Cost is computed from token counts and a pricing table maintained per model. Attribution fields in the request let you break costs down by caller, model, route group, or any combination. Budget enforcement can reject requests before they're sent when a caller's spend would exceed their configured limit.

## How token counting works

For providers that return token counts in the response (OpenAI, Anthropic, most hosted APIs), infermux reads `usage.prompt_tokens` and `usage.completion_tokens` from the response body. For providers that don't return usage (some local deployments, streaming responses mid-stream), infermux estimates token counts using a character-count approximation: 1 token per 4 characters for the prompt, with the completion counted after the response is complete.

Streaming responses are handled correctly: for OpenAI-compatible providers using SSE streaming, infermux buffers the final `data: [DONE]` chunk which includes the usage summary. If no usage summary is provided in the stream, infermux falls back to character estimation.

## Pricing tables

infermux ships with pricing tables for all supported providers. Prices are stored as cost per million tokens (input and output separately):

```yaml
# Built-in pricing table (excerpt)
# You can override any entry in infermux.yml

pricing:
  openai:
    gpt-4o:
      input_per_million: 2.50
      output_per_million: 10.00
    gpt-4o-mini:
      input_per_million: 0.15
      output_per_million: 0.60
  anthropic:
    claude-opus-4-5:
      input_per_million: 3.00
      output_per_million: 15.00
    claude-haiku-3-5:
      input_per_million: 0.80
      output_per_million: 4.00
```

To override pricing in your config:

```yaml
pricing:
  openai:
    gpt-4o:
      input_per_million: 2.50    # your negotiated rate
      output_per_million: 10.00
  local-vllm:
    Llama-3.1-8B-Instruct:
      input_per_million: 0.0     # self-hosted: zero API cost
      output_per_million: 0.0
```

Cost for a request is: `(prompt_tokens / 1_000_000 * input_per_million) + (completion_tokens / 1_000_000 * output_per_million)`.

## Cost attribution

Each request can carry attribution metadata that infermux uses to tag cost events. Attribution is carried in request headers:

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Infermux-Caller: user-abc123" \
  -H "X-Infermux-Project: summarization-pipeline" \
  -d '{"model": "gpt-4o-mini", "messages": [...]}'
```

Available attribution headers:

| Header | Description |
|--------|-------------|
| `X-Infermux-Caller` | Caller identity (user ID, service name, etc.) |
| `X-Infermux-Project` | Project or pipeline name |
| `X-Infermux-Env` | Environment (production, staging, dev) |

When using infermux as a library, attribution is set on the request context:

```go
import "github.com/greynewell/infermux"

ctx = infermux.WithAttribution(ctx, infermux.Attribution{
    Caller:  "user-abc123",
    Project: "summarization-pipeline",
    Env:     "production",
})
```

All cost events include the attribution fields, provider name, model name, prompt tokens, completion tokens, and computed cost in USD.

## Budget enforcement

Set per-caller spend budgets in the config. Budgets are evaluated per calendar month in UTC:

```yaml
budgets:
  - caller: "user-abc123"
    monthly_usd: 10.00
    action: hard_stop      # reject requests that would exceed budget

  - caller: "user-def456"
    monthly_usd: 50.00
    action: alert          # allow requests but emit an alert event

  - caller: "batch-pipeline"
    monthly_usd: 500.00
    action: hard_stop
    model_override: "gpt-4o-mini"   # downgrade model instead of rejecting
```

**hard_stop**: When a request would push a caller's spend over their monthly budget, infermux returns a 429 with `X-Infermux-Error: budget_exceeded` before the request reaches the provider. The current spend and limit are included in the response body.

**alert**: The request proceeds normally. An alert event is emitted on the mist-go metrics bus. If you have tokentrace configured, this triggers an alert notification.

**model_override**: Instead of rejecting, infermux rewrites the request to use a cheaper model. The caller receives a response from the cheaper model. The `X-Infermux-Model-Override: gpt-4o-mini` header in the response indicates the substitution was made.

Budget state is stored in memory and does not persist across restarts. For persistent budget tracking, use the HTTP API to read current spend and manage budgets externally, or configure a `state_file` path:

```yaml
budgets:
  state_file: "/var/lib/infermux/budget-state.json"
```

## Cost reports via the HTTP API

```bash
# Total spend by provider for the current month
curl http://localhost:8080/_infermux/costs

# Spend breakdown by caller
curl http://localhost:8080/_infermux/costs/by-caller

# Spend for a specific caller
curl http://localhost:8080/_infermux/costs/caller/user-abc123

# Spend by model across all providers
curl http://localhost:8080/_infermux/costs/by-model

# Time-series data for the last 24 hours (hourly buckets)
curl "http://localhost:8080/_infermux/costs/timeseries?hours=24&bucket=1h"
```

Example response from `/costs`:

```json
{
  "period": "2026-03",
  "total_usd": 14.82,
  "by_provider": {
    "openai": 11.40,
    "anthropic": 2.91,
    "ollama": 0.00
  },
  "by_model": {
    "gpt-4o": 9.12,
    "gpt-4o-mini": 2.28,
    "claude-haiku-3-5": 2.91,
    "llama3.2": 0.00
  },
  "requests": 48291,
  "prompt_tokens": 18400200,
  "completion_tokens": 4200100
}
```
