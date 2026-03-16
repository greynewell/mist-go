---
title: "Configuration"
description: "Complete infermux.yml schema. Every field, type, and default. Worked examples."
---

# Configuration

infermux is configured with a single YAML file, typically named `infermux.yml`. All values support `${ENV_VAR}` substitution. Fields marked as optional have the listed default when omitted.

## Complete schema

```yaml
# infermux.yml

# listen specifies the address and port for the inference API.
# Default: ":8080"
listen: ":8080"

# management_listen is a separate listener for management endpoints (/_infermux/*).
# Optional. If omitted, management endpoints are served on the same port as the inference API.
management_listen: ":8081"

# log configures structured logging.
log:
  level: info          # debug | info | warn | error. Default: info
  format: text         # text | json. Default: text
  timestamps: true     # include timestamps. Default: true

# tls configures TLS for the inference API listener.
# Optional. If omitted, the server listens on plain HTTP.
tls:
  cert_file: "/path/to/cert.pem"
  key_file:  "/path/to/key.pem"

# auth configures how infermux authenticates incoming inference requests.
auth:
  mode: passthrough    # passthrough | static_key. Default: passthrough
  key: "${INFERMUX_API_KEY}"  # required when mode is static_key

# providers is the list of LLM providers infermux can route to.
# At least one provider is required.
providers:
  - name: openai                       # required; unique across providers
    type: openai                       # openai | anthropic | ollama | azure_openai
    api_key: "${OPENAI_API_KEY}"       # required for hosted providers
    base_url: "https://api.openai.com/v1"  # optional; override default endpoint
    timeout: 60s                       # per-request timeout. Default: 60s
    models:                            # optional; restrict which models this provider serves
      - gpt-4o
      - gpt-4o-mini
    model_aliases:                     # optional; map caller model names to provider model names
      custom-name: gpt-4o
    rate_limits:                       # optional; infermux tracks these and avoids exceeding them
      requests_per_minute: 3500
      tokens_per_minute: 800000
    health_check:
      interval: 60s                    # Default: 60s
      timeout: 10s                     # Default: 10s
      disabled: false                  # Default: false
    circuit_breaker:                   # optional; overrides global circuit_breaker settings
      error_rate_threshold: 0.5
      consecutive_failures: 5
      latency_p95_ms: 5000
      window_seconds: 60
      min_requests: 10
      recovery_window: 30s
      probe_timeout: 5s
      recovery_backoff_multiplier: 1.0  # 1.0 = no backoff. Default: 1.0
      recovery_backoff_max: 600s

# global circuit_breaker defaults, applied to all providers that don't override them.
circuit_breaker:
  error_rate_threshold: 0.5
  consecutive_failures: 5
  latency_p95_ms: 5000
  window_seconds: 60
  min_requests: 10
  recovery_window: 30s
  probe_timeout: 5s
  treat_rate_limit_as_error: false

# routing configures how infermux selects providers.
routing:
  strategy: round_robin    # round_robin | least_latency | cost_weighted | random | priority

  # least_latency options (only used when strategy is least_latency)
  least_latency:
    ewma_decay: 0.1
    min_samples: 5

  # groups override the top-level strategy for specific models.
  groups:
    - name: group-name      # required; used in logs and headers
      models:               # models that route through this group
        - gpt-4o-mini
      strategy: cost_weighted
      providers:            # optional; restrict to these providers for this group
        - openai
        - anthropic

# pricing overrides for the built-in pricing tables.
# Optional. Useful for negotiated rates or self-hosted models.
pricing:
  openai:
    gpt-4o:
      input_per_million: 2.50
      output_per_million: 10.00

# budgets enforce per-caller monthly spend limits.
budgets:
  state_file: ""             # optional; persist budget state to this path
  - caller: "user-abc123"
    monthly_usd: 10.00
    action: hard_stop        # hard_stop | alert | model_override
    model_override: ""       # model to use when action is model_override
```

## Worked examples

### Single provider (minimal)

```yaml
listen: ":8080"

providers:
  - name: openai
    type: openai
    api_key: "${OPENAI_API_KEY}"

routing:
  strategy: round_robin
```

### Multi-provider with priority failover

```yaml
listen: ":8080"

providers:
  - name: openai
    type: openai
    api_key: "${OPENAI_API_KEY}"
    models:
      - gpt-4o
      - gpt-4o-mini
    rate_limits:
      requests_per_minute: 3500
      tokens_per_minute: 800000

  - name: anthropic
    type: anthropic
    api_key: "${ANTHROPIC_API_KEY}"
    model_aliases:
      gpt-4o: claude-opus-4-5
      gpt-4o-mini: claude-haiku-3-5

circuit_breaker:
  error_rate_threshold: 0.4
  consecutive_failures: 5
  recovery_window: 30s

routing:
  strategy: priority
```

### Cost-optimized routing with route groups

```yaml
listen: ":8080"

providers:
  - name: openai
    type: openai
    api_key: "${OPENAI_API_KEY}"
    models: [gpt-4o, gpt-4o-mini, text-embedding-3-small]

  - name: anthropic
    type: anthropic
    api_key: "${ANTHROPIC_API_KEY}"
    model_aliases:
      gpt-4o: claude-opus-4-5
      gpt-4o-mini: claude-haiku-3-5

  - name: groq
    type: openai
    api_key: "${GROQ_API_KEY}"
    base_url: "https://api.groq.com/openai/v1"
    models:
      - llama-3.1-70b-versatile
    model_aliases:
      gpt-4o-mini: llama-3.1-70b-versatile
    rate_limits:
      requests_per_minute: 30

  - name: ollama
    type: ollama
    base_url: "http://localhost:11434"
    models: [llama3.2]
    health_check:
      interval: 10s

routing:
  strategy: cost_weighted

  groups:
    - name: embeddings
      models: [text-embedding-3-small, text-embedding-3-large]
      strategy: round_robin
      providers: [openai]

    - name: reasoning
      models: [gpt-4o, claude-opus-4-5]
      strategy: priority
      providers: [openai, anthropic]

budgets:
  state_file: "/var/lib/infermux/budgets.json"
  - caller: "batch-pipeline"
    monthly_usd: 200.00
    action: model_override
    model_override: "gpt-4o-mini"
```

### Production with TLS, JSON logging, and management on a separate port

```yaml
listen: ":443"
management_listen: ":8081"

tls:
  cert_file: "/etc/ssl/infermux.crt"
  key_file:  "/etc/ssl/infermux.key"

log:
  level: info
  format: json
  timestamps: true

auth:
  mode: static_key
  key: "${INFERMUX_API_KEY}"

providers:
  - name: openai
    type: openai
    api_key: "${OPENAI_API_KEY}"
    models: [gpt-4o, gpt-4o-mini]
    timeout: 45s
    rate_limits:
      requests_per_minute: 3500
      tokens_per_minute: 800000

  - name: anthropic
    type: anthropic
    api_key: "${ANTHROPIC_API_KEY}"
    model_aliases:
      gpt-4o: claude-opus-4-5
      gpt-4o-mini: claude-haiku-3-5
    timeout: 60s

circuit_breaker:
  error_rate_threshold: 0.3
  consecutive_failures: 5
  recovery_window: 60s
  recovery_backoff_multiplier: 2.0
  recovery_backoff_max: 600s

routing:
  strategy: priority
```
