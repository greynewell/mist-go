---
title: "Optimize inference costs with routing strategies"
description: "Route to the cheapest provider, set per-caller spend caps, downgrade models on budget, and read cost reports."
difficulty: intermediate
duration: "25 min"
---

# Optimize inference costs with routing strategies

<div class="tutorial-meta">
  <span class="meta-tag">intermediate</span>
  <span class="meta-tag">25 min</span>
</div>

In this tutorial you'll configure infermux to minimize inference costs across three providers, set monthly spend caps per caller, and read cost reports from the management API. By the end you'll have a config that routes cheap tasks to cheap models and reserves expensive models for where they're needed.

**What you need:**
- infermux installed
- An OpenAI API key
- An Anthropic API key
- Ollama running locally (optional — used for the zero-cost tier)
- `curl` and `jq`

---

<div class="step">
<div class="step-number">Step 1</div>

## Understand the cost-weighted strategy

The `cost_weighted` routing strategy estimates the cost of each request before it's sent and selects the provider expected to minimize that cost. Cost is estimated from the prompt token count and the built-in pricing table.

With three providers configured, cost-weighted routing will prefer:
1. Ollama (local) — $0.00 per token
2. Anthropic Claude Haiku — cheapest hosted option for most models
3. OpenAI GPT-4o-mini — next cheapest
4. More expensive models — only when explicitly requested

You get this behavior without any application code changes.

</div>

<div class="step">
<div class="step-number">Step 2</div>

## Set up Ollama (optional zero-cost tier)

If you have a machine with enough RAM to run a local model, install Ollama and pull a model:

```bash
# macOS
brew install ollama
ollama serve &
ollama pull llama3.2
```

Verify it's running:

```bash
curl http://localhost:11434/api/tags | jq '.models[].name'
# "llama3.2:latest"
```

Skip this step if you don't have Ollama — the tutorial works without it.

</div>

<div class="step">
<div class="step-number">Step 3</div>

## Write the config

```bash
export OPENAI_API_KEY=sk-proj-...
export ANTHROPIC_API_KEY=sk-ant-...
```

Create `infermux.yml`:

```yaml
listen: ":8080"
management_listen: ":8081"

providers:
  - name: openai
    type: openai
    api_key: "${OPENAI_API_KEY}"
    models:
      - gpt-4o
      - gpt-4o-mini
      - text-embedding-3-small

  - name: anthropic
    type: anthropic
    api_key: "${ANTHROPIC_API_KEY}"
    model_aliases:
      gpt-4o: claude-opus-4-5
      gpt-4o-mini: claude-haiku-3-5

  # Remove this block if you don't have Ollama
  - name: ollama
    type: ollama
    base_url: "http://localhost:11434"
    models: [llama3.2]
    model_aliases:
      gpt-4o-mini: llama3.2
    health_check:
      interval: 10s
      timeout: 3s

routing:
  strategy: cost_weighted

  groups:
    # Embeddings only go to OpenAI (Anthropic doesn't support them)
    - name: embeddings
      models: [text-embedding-3-small, text-embedding-3-large]
      strategy: round_robin
      providers: [openai]

    # Complex reasoning: priority order, most capable models
    - name: reasoning
      models: [gpt-4o, claude-opus-4-5]
      strategy: priority
      providers: [openai, anthropic]

log:
  level: info
  format: json
```

The `groups` section routes embeddings to OpenAI only (they don't need cost comparison since only one provider supports them), and keeps complex reasoning on priority routing so it always goes to the best available model rather than the cheapest.

Everything else — `gpt-4o-mini` requests — will route cost-weighted across all three providers.

</div>

<div class="step">
<div class="step-number">Step 4</div>

## Start infermux and verify cost-weighted routing

```bash
infermux serve --config infermux.yml
```

Send several `gpt-4o-mini` requests and watch which provider serves them:

```bash
for i in $(seq 1 6); do
  curl -s -D - http://localhost:8080/v1/chat/completions \
    -H "Content-Type: application/json" \
    -d '{"model": "gpt-4o-mini", "messages": [{"role": "user", "content": "hi"}]}' \
    | grep -E "X-Infermux-Provider|X-Infermux-Cost"
  echo "---"
done
```

With Ollama configured, you'll see it preferred because its cost is $0.00. Without Ollama, you'll see requests split between Anthropic (claude-haiku-3-5) and OpenAI (gpt-4o-mini) based on which is cheaper per request.

</div>

<div class="step">
<div class="step-number">Step 5</div>

## Add per-caller budgets

Update `infermux.yml` to add budgets:

```yaml
budgets:
  state_file: "/tmp/infermux-budgets.json"

  - caller: "free-tier"
    monthly_usd: 1.00
    action: hard_stop

  - caller: "pro-tier"
    monthly_usd: 25.00
    action: alert

  - caller: "batch-pipeline"
    monthly_usd: 50.00
    action: model_override
    model_override: "gpt-4o-mini"
```

Restart infermux, then test the budget enforcement. Pass the caller identity in the `X-Infermux-Caller` header:

```bash
curl -s http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Infermux-Caller: free-tier" \
  -d '{"model": "gpt-4o-mini", "messages": [{"role": "user", "content": "hi"}]}'
```

This works normally because the free-tier caller hasn't spent anything yet.

</div>

<div class="step">
<div class="step-number">Step 6</div>

## Simulate budget exhaustion

You can test the hard-stop behavior by setting the budget to a very small amount. Temporarily update the free-tier budget to $0.000001 (so the first request exhausts it) and restart infermux:

```yaml
  - caller: "free-tier"
    monthly_usd: 0.000001
    action: hard_stop
```

Now send a request:

```bash
curl -s http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Infermux-Caller: free-tier" \
  -d '{"model": "gpt-4o-mini", "messages": [{"role": "user", "content": "hi"}]}'
```

```json
{
  "error": {
    "message": "monthly budget of $0.000001 exceeded; current spend: $0.0000018",
    "type": "infermux_error",
    "code": "budget_exceeded"
  }
}
```

The request was rejected before it reached any provider. Reset the budget to $1.00 for normal testing.

</div>

<div class="step">
<div class="step-number">Step 7</div>

## Read cost reports

After sending some requests, query the cost API:

```bash
# Overall spend by provider
curl -s http://localhost:8081/_infermux/costs | jq .
```

```json
{
  "period": "2026-03",
  "total_usd": 0.0000842,
  "by_provider": {
    "openai": 0.0000421,
    "anthropic": 0.0000421,
    "ollama": 0.0
  },
  "requests": 12
}
```

```bash
# Spend by model
curl -s http://localhost:8081/_infermux/costs/by-model | jq '.by_model'
```

```json
{
  "gpt-4o-mini": 0.0000421,
  "claude-haiku-3-5": 0.0000421,
  "llama3.2": 0.0
}
```

```bash
# Spend by caller
curl -s http://localhost:8081/_infermux/costs/by-caller | jq '.callers'
```

```json
[
  {
    "caller": "free-tier",
    "total_usd": 0.0000018,
    "budget_usd": 1.0,
    "budget_remaining_usd": 0.9999982,
    "requests": 1
  }
]
```

Callers without a configured budget appear in the spend report without budget fields.

</div>

<div class="step">
<div class="step-number">Step 8</div>

## Calculate your savings

To see how much cost-weighted routing saves compared to sending everything to OpenAI GPT-4o, compare the costs:

| Approach | Per request (typical) | Per 100k requests/day |
|----------|-----------------------|-----------------------|
| All OpenAI GPT-4o | $0.0020 | $200/day |
| All OpenAI GPT-4o-mini | $0.00015 | $15/day |
| Cost-weighted (Haiku/mini/Ollama mix) | $0.00008 | $8/day |
| Cost-weighted with local Ollama primary | ~$0.000002 | ~$0.20/day |

The actual savings depend on your prompt lengths and usage patterns. Longer prompts favor Ollama more strongly (token counts are higher, so the $0 cost advantage is larger). Short prompts make the difference between Haiku and gpt-4o-mini relatively small in absolute terms.

</div>

<div class="callout note">

**Match model capability to task requirements before optimizing cost.** The cheapest option is only valuable if the output quality is sufficient. Run an eval with matchspec on a representative sample before routing production traffic to a cheaper model. A 94% cost reduction is worthless if it causes a 30% quality regression on your task.

</div>

## What you built

A cost-optimized infermux config that routes cheap tasks to cheap models using the cost-weighted strategy, reserves complex reasoning for priority-ordered expensive models, and enforces per-caller monthly spend caps.

## What's next

- [Cost Tracking](/infermux/docs/cost-tracking/) — full reference for pricing tables, attribution, and budget enforcement
- [Cost Optimization guide](/infermux/docs/cost-optimization/) — deeper analysis and real-world numbers
- [Routing](/infermux/docs/routing/) — all routing strategies and route group configuration
