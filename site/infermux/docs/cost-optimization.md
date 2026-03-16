---
title: "Cost Optimization"
description: "Route by cost, set spend caps, downgrade models on budget, and read cost reports. Real-world example with numbers."
---

# Cost Optimization

LLM inference costs add up fast. A production system handling 100k requests per day at $0.002 average per request is $200/day — $6,000/month. infermux gives you three levers to control this: **routing strategy** (send requests to the cheapest eligible provider), **model routing** (use expensive models only where they matter), and **budgets** (cap per-caller spend before it reaches the provider).

## Routing to the cheapest provider

The `cost_weighted` strategy selects the provider expected to minimize cost for each request:

```yaml
routing:
  strategy: cost_weighted
```

Cost is estimated before the request is sent using the prompt token count and the pricing table. Completion tokens are unknown before the response, so the estimate uses only prompt tokens for selection. This is a reasonable approximation: prompt tokens account for most of the cost on long-context requests, and for short prompts the absolute dollar difference between providers is small.

With three providers configured — OpenAI, Anthropic, and Ollama (local) — the cost-weighted strategy will route to Ollama for any model that has a zero-cost entry in the pricing table, then to the cheapest hosted provider.

## Route expensive models selectively

The most impactful optimization is using a cheap model for tasks that don't need a powerful one. Use route groups to send different model tiers to different providers and strategies:

```yaml
routing:
  strategy: priority    # fallback for unmatched requests

  groups:
    # High-volume simple tasks: cost-optimized, cheap models
    - name: fast
      models:
        - gpt-4o-mini
        - claude-haiku-3-5
      strategy: cost_weighted
      providers: [openai, anthropic, groq]

    # Complex reasoning: priority routing, expensive models
    - name: reasoning
      models:
        - gpt-4o
        - o1
        - claude-opus-4-5
      strategy: priority
      providers: [openai, anthropic]

    # Embeddings: always route to cheapest available
    - name: embeddings
      models:
        - text-embedding-3-small
      strategy: cost_weighted
      providers: [openai]
```

With this config, simple chat requests (gpt-4o-mini) are routed cost-optimally across three providers. Complex reasoning requests always go to OpenAI first, falling back to Anthropic. Embeddings are handled separately and always use the cheapest available option.

## Real-world example with numbers

Consider a summarization pipeline that processes 50,000 documents per day. Each document averages 2,000 input tokens and 200 output tokens.

**Without cost optimization** — all requests to OpenAI GPT-4o:
- 50,000 × 2,000 input tokens = 100M input tokens/day
- 50,000 × 200 output tokens = 10M output tokens/day
- Cost: (100M / 1M × $2.50) + (10M / 1M × $10.00) = $250 + $100 = **$350/day**

**With cost optimization** — route summarization to GPT-4o-mini:
- Same volumes
- Cost: (100M / 1M × $0.15) + (10M / 1M × $0.60) = $15 + $6 = **$21/day**

That is a 94% cost reduction on this workload. The tradeoff is output quality — which you verify with matchspec on a holdout set before switching.

**With local Ollama** — route to llama3.2 locally:
- Cost: $0 (server electricity only)

In practice a realistic optimization is: route 80% of traffic (simple queries, short contexts) to a cheap hosted model or local model, and keep 20% (long contexts, complex reasoning, high-stakes outputs) on GPT-4o. If your average cost before optimization is $0.002/request, a routing strategy that achieves 80/20 splits gets you to roughly $0.0005/request — a 75% reduction.

## Per-caller budgets

For multi-tenant systems where different callers should have different spend caps:

```yaml
budgets:
  state_file: "/var/lib/infermux/budgets.json"

  - caller: "free-tier-user"
    monthly_usd: 1.00
    action: hard_stop

  - caller: "pro-user"
    monthly_usd: 20.00
    action: alert

  - caller: "enterprise-user"
    monthly_usd: 500.00
    action: alert

  - caller: "batch-pipeline"
    monthly_usd: 100.00
    action: model_override
    model_override: "gpt-4o-mini"   # downgrade instead of rejecting
```

The `model_override` action is particularly useful for batch pipelines: when the budget nears the limit, infermux transparently downgrades to a cheaper model rather than failing the request. The caller receives a response with `X-Infermux-Model-Override: gpt-4o-mini` to signal the substitution.

Pass the caller identity in the request header:

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "X-Infermux-Caller: free-tier-user" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4o", "messages": [...]}'
```

When the free-tier-user's $1.00 monthly budget is exhausted:

```json
HTTP/1.1 429 Too Many Requests
X-Infermux-Error: budget_exceeded

{
  "error": {
    "message": "monthly budget of $1.00 exceeded; current spend: $1.003",
    "type": "infermux_error",
    "code": "budget_exceeded"
  }
}
```

## Reading cost reports

The management API gives you current-month cost data broken down multiple ways:

```bash
# Total by provider
curl http://localhost:8081/_infermux/costs

# By caller — see which callers are driving spend
curl http://localhost:8081/_infermux/costs/by-caller

# By model — identify expensive model usage
curl http://localhost:8081/_infermux/costs/by-model

# Hourly time series for the last 24 hours
curl "http://localhost:8081/_infermux/costs/timeseries?hours=24&bucket=1h"
```

Example `/costs/by-caller` output:

```json
{
  "period": "2026-03",
  "callers": [
    {
      "caller": "batch-pipeline",
      "total_usd": 84.20,
      "budget_usd": 100.00,
      "budget_remaining_usd": 15.80,
      "requests": 42100,
      "prompt_tokens": 84200000,
      "completion_tokens": 8420000
    },
    {
      "caller": "user-abc123",
      "total_usd": 0.83,
      "budget_usd": 1.00,
      "budget_remaining_usd": 0.17,
      "requests": 415
    }
  ]
}
```

## Connecting to tokentrace

When running the full MIST stack, infermux emits cost events as mist-go metrics that tokentrace collects. This gives you cost data in your existing observability stack alongside latency and quality metrics:

```yaml
# tokentrace.yml
sources:
  - type: infermux
    address: "http://localhost:8080"

alerts:
  - name: daily-spend-spike
    metric: infermux.cost_usd
    condition: "rate_1h > 15"   # alert if hourly spend exceeds $15
    severity: warning
```

With this setup, you get spend data in Grafana dashboards, time-series cost queries via tokentrace's HTTP API, and alerts when spend spikes unexpectedly.
