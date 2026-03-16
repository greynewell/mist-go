---
title: "Routing"
description: "Routing strategies: round-robin, least-latency, cost-weighted, random, sticky-session. Route groups and priority routing."
---

# Routing

The router selects a target provider for each incoming request. The selection process runs in two phases: first, it filters the provider set to only healthy, eligible providers (those that serve the requested model and have open circuits and available rate limit headroom); second, it applies the configured strategy to select from the eligible set. If the eligible set is empty after filtering, infermux returns a 503 with a `X-Infermux-Error: no_healthy_providers` header.

## Configuring a strategy

```yaml
routing:
  strategy: least_latency   # round_robin | least_latency | cost_weighted | random | priority
```

The strategy applies globally to all routes unless overridden by a route group.

## round_robin

Distributes requests evenly across healthy providers in rotation. This is the default strategy and the simplest to reason about.

```yaml
routing:
  strategy: round_robin
```

Round-robin is a good default when your providers are roughly equivalent in cost and latency, or when you want predictable load distribution for capacity planning.

## least_latency

Selects the provider with the lowest EWMA (exponentially-weighted moving average) of response time. The EWMA uses a decay factor of 0.1, which means recent measurements carry significantly more weight than older ones. A new provider with no history starts with a latency of zero and will be preferred until it accumulates enough history for the EWMA to stabilize.

```yaml
routing:
  strategy: least_latency
  least_latency:
    ewma_decay: 0.1          # optional; default 0.1
    min_samples: 5           # optional; minimum requests before EWMA is trusted
```

With `min_samples` set, a provider with fewer than 5 completed requests is treated as having the lowest possible latency for the purpose of selection (it will be preferred). This ensures new providers warm up quickly.

## cost_weighted

Selects the provider expected to minimize cost for the current request. Cost is estimated before the request completes using the prompt token count (counted from the request body) and the pricing table entry for the target model. If prompt token counts are not provided in the request, infermux estimates them using a character-count approximation (1 token ≈ 4 characters).

```yaml
routing:
  strategy: cost_weighted
```

The cost-weighted strategy requires that your providers have models with entries in the pricing tables. Providers with unknown pricing are treated as having zero cost and will always be preferred. To update or override pricing, see the [cost tracking](/infermux/docs/cost-tracking/) documentation.

## random

Selects uniformly at random from the healthy eligible set. Useful when you want to avoid any correlation between consecutive requests and don't need load balancing properties.

```yaml
routing:
  strategy: random
```

## priority

Always tries providers in the order they are declared in the `providers` list. The first healthy provider that serves the requested model receives the request. This is the right strategy when you have a clear primary provider and want others to be strict fallbacks.

```yaml
routing:
  strategy: priority
```

With two providers, requests always go to the first one until its circuit opens, at which point they automatically shift to the second. When the first provider recovers and its circuit closes, it resumes receiving traffic immediately.

## Route groups

Route groups let you override the routing strategy for specific models or request patterns. This allows cost-optimized routing for cheap tasks while using priority routing for expensive models.

```yaml
routing:
  strategy: round_robin     # default for requests not matched by any group

  groups:
    - name: fast-tasks
      models:
        - gpt-4o-mini
        - claude-haiku-3-5
      strategy: cost_weighted
      providers:
        - openai
        - anthropic
        - groq

    - name: reasoning
      models:
        - gpt-4o
        - claude-opus-4-5
        - o1
      strategy: priority
      providers:
        - openai
        - anthropic

    - name: embeddings
      models:
        - text-embedding-3-small
        - text-embedding-3-large
      strategy: round_robin
      providers:
        - openai
```

Route groups match by model name (after alias expansion). The first group whose `models` list contains the requested model wins. If no group matches, the top-level strategy and provider set apply.

The `providers` list in a route group restricts which providers are eligible for that group. Providers not in the list are not considered for requests matched by the group, even if they are healthy and serve the requested model.

## Priority routing in detail

Priority routing is the most important strategy to understand because it is the basis for failover. When you configure `strategy: priority`, infermux tries providers in declaration order. It does not round-robin or randomize. The first eligible provider always wins.

This interacts with circuit breaking as follows: if provider A has its circuit open (due to recent failures), it is ineligible. infermux skips it and selects provider B. When A's circuit closes after recovery, it becomes eligible again and immediately resumes receiving traffic as the priority-1 provider.

To implement "primary plus fallback" you don't need any special configuration beyond ordering:

```yaml
providers:
  - name: openai          # tried first
    type: openai
    api_key: "${OPENAI_API_KEY}"
    models: [gpt-4o, gpt-4o-mini]

  - name: anthropic       # fallback if openai circuit is open
    type: anthropic
    api_key: "${ANTHROPIC_API_KEY}"
    model_aliases:
      gpt-4o: claude-opus-4-5
      gpt-4o-mini: claude-haiku-3-5

routing:
  strategy: priority
```

## How routing decisions are logged

Every request adds routing information to the response headers and to the structured log output:

```
X-Infermux-Provider: openai
X-Infermux-Model: gpt-4o-mini
X-Infermux-Strategy: least_latency
X-Infermux-Route-Group: fast-tasks
X-Infermux-Latency-Ms: 342
X-Infermux-Cost: 0.000024
```

The structured log line for each request includes all routing fields as JSON, which makes it straightforward to build dashboards showing which provider serves what fraction of traffic, and how routing decisions shift as providers become healthy or unhealthy.
