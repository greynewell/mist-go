---
title: "Providers"
description: "Configure OpenAI, Anthropic, Ollama, Azure OpenAI, and custom providers. Health checks, model aliases, and rate limits."
---

# Providers

A provider is a named LLM endpoint with credentials, model declarations, and routing metadata. Providers are declared in `infermux.yml` under the `providers` key. infermux loads all providers at startup, runs an initial health check against each, and begins routing based on their health state.

## Common configuration fields

Every provider, regardless of type, supports these fields:

```yaml
providers:
  - name: my-provider          # required; unique name used in logs and headers
    type: openai               # required; see supported types below
    api_key: "${MY_API_KEY}"   # required for hosted providers; supports env vars
    base_url: "https://..."    # optional; overrides the default endpoint
    timeout: 30s               # optional; per-request timeout (default: 60s)
    models:                    # optional; restrict which models this provider serves
      - gpt-4o
      - gpt-4o-mini
    model_aliases:             # optional; map caller model names to provider model names
      gpt-4o: claude-opus-4-5
    rate_limits:
      requests_per_minute: 500
      tokens_per_minute: 100000
    health_check:
      interval: 30s            # how often to probe (default: 60s)
      timeout: 5s              # probe timeout (default: 10s)
      disabled: false          # set true to skip health checks entirely
```

## OpenAI

```yaml
providers:
  - name: openai
    type: openai
    api_key: "${OPENAI_API_KEY}"
    models:
      - gpt-4o
      - gpt-4o-mini
      - gpt-3.5-turbo
      - text-embedding-3-small
      - text-embedding-3-large
    rate_limits:
      requests_per_minute: 3500
      tokens_per_minute: 800000
```

The `openai` provider type uses `https://api.openai.com/v1` as its base URL. Override `base_url` to point at any OpenAI-compatible endpoint.

## Anthropic

```yaml
providers:
  - name: anthropic
    type: anthropic
    api_key: "${ANTHROPIC_API_KEY}"
    model_aliases:
      gpt-4o: claude-opus-4-5
      gpt-4o-mini: claude-haiku-3-5
      gpt-3.5-turbo: claude-haiku-3-5
    rate_limits:
      requests_per_minute: 1000
      tokens_per_minute: 400000
```

Anthropic's API is not OpenAI-compatible natively. infermux translates between the two formats: it converts OpenAI chat completion requests to Anthropic's Messages API format, and converts responses back. Model aliases are required if you want to use OpenAI model names in requests; infermux will translate them before forwarding.

## Ollama (local)

```yaml
providers:
  - name: ollama
    type: ollama
    base_url: "http://localhost:11434"
    models:
      - llama3.2
      - mistral
      - qwen2.5-coder
    health_check:
      interval: 10s
      timeout: 3s
```

Ollama exposes an OpenAI-compatible API at `/v1`. The `ollama` type is equivalent to `openai` with the base URL set to your Ollama instance. No `api_key` is required.

## Azure OpenAI

```yaml
providers:
  - name: azure-openai
    type: azure_openai
    api_key: "${AZURE_OPENAI_API_KEY}"
    base_url: "https://my-resource.openai.azure.com"
    azure_deployment: "gpt-4o-deployment"
    azure_api_version: "2024-08-01-preview"
    models:
      - gpt-4o
      - gpt-4o-mini
```

Azure OpenAI uses a different URL structure (`/openai/deployments/{deployment}/chat/completions`) and requires `api-version` as a query parameter. The `azure_openai` type handles this automatically. Set `azure_deployment` to your Azure deployment name and `azure_api_version` to the API version you've configured.

## Custom OpenAI-compatible providers

Any endpoint that speaks the OpenAI chat completions API can be added as a custom provider using `type: openai` with a custom `base_url`:

```yaml
providers:
  - name: groq
    type: openai
    api_key: "${GROQ_API_KEY}"
    base_url: "https://api.groq.com/openai/v1"
    models:
      - llama-3.1-70b-versatile
      - mixtral-8x7b-32768
    rate_limits:
      requests_per_minute: 30
      tokens_per_minute: 6000

  - name: together
    type: openai
    api_key: "${TOGETHER_API_KEY}"
    base_url: "https://api.together.xyz/v1"
    models:
      - meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo
      - mistralai/Mixtral-8x7B-Instruct-v0.1

  - name: local-vllm
    type: openai
    base_url: "http://10.0.1.5:8000/v1"
    models:
      - Llama-3.1-8B-Instruct
    health_check:
      interval: 5s
```

## Provider health checks

infermux probes each provider at startup and on the configured `health_check.interval`. The probe sends a minimal request — for chat providers, this is typically a single-token generation with `max_tokens: 1`. If the probe returns a 2xx response within `health_check.timeout`, the provider is marked healthy. Otherwise it is marked unhealthy and excluded from routing.

Health state is separate from circuit breaker state. A provider can be circuit-open (temporarily excluded due to recent failures in live traffic) while still passing health checks. The router uses the intersection: only providers that are both health-check-passing and circuit-closed receive traffic.

To disable health checks entirely for a provider (useful for providers with rate limits that you don't want to consume on probes):

```yaml
providers:
  - name: expensive-provider
    type: openai
    api_key: "${API_KEY}"
    health_check:
      disabled: true
```

## Provider metadata used in routing

The router uses four pieces of metadata from the provider registry when making routing decisions:

- **Health state** — open circuits and unhealthy providers are filtered out before any strategy runs.
- **Models** — if the provider's `models` list is set, the router only considers providers that list the requested model (or have a model alias for it).
- **Rate limits** — infermux tracks request and token counts per provider and skips providers that are at their configured rate limit ceiling. Limits reset on a 60-second rolling window.
- **Latency history** — the `least_latency` strategy reads an exponentially-weighted moving average (EWMA) of response times maintained per provider. This is updated after every successful request.
