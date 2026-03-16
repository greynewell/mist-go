---
title: "Set up automatic failover between OpenAI and Anthropic"
description: "Configure OpenAI as primary and Anthropic as fallback. Tune circuit breaker thresholds, simulate failures, and verify automatic recovery."
difficulty: intermediate
duration: "20 min"
---

# Set up automatic failover between OpenAI and Anthropic

<div class="tutorial-meta">
  <span class="meta-tag">intermediate</span>
  <span class="meta-tag">20 min</span>
</div>

In this tutorial you'll configure infermux with OpenAI as the primary provider and Anthropic as an automatic fallback. You'll tune the circuit breaker so it responds appropriately to failures, simulate a provider outage, and verify that requests shift to Anthropic automatically. You'll also learn how to monitor circuit state and verify recovery when the primary comes back.

**What you need:**
- infermux installed (`go install github.com/greynewell/infermux/cmd/infermux@latest`)
- An OpenAI API key
- An Anthropic API key
- `curl` and `jq`

---

<div class="step">
<div class="step-number">Step 1</div>

## Understand priority routing

infermux's `priority` routing strategy always tries providers in declaration order. The first healthy provider with a closed circuit that serves the requested model wins. If OpenAI's circuit is open, infermux skips it and selects Anthropic. When OpenAI recovers, it resumes receiving traffic immediately.

This is the foundation of failover: you declare providers in priority order, configure circuit breaker thresholds appropriate for your SLA, and infermux handles the rest.

</div>

<div class="step">
<div class="step-number">Step 2</div>

## Write the config

Set your API keys:

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
    timeout: 30s
    circuit_breaker:
      error_rate_threshold: 0.5    # open when 50% of requests fail
      consecutive_failures: 5      # or after 5 consecutive failures
      window_seconds: 30           # measured over a 30-second window
      min_requests: 5              # require at least 5 requests before evaluating rate
      recovery_window: 20s         # stay open 20 seconds, then probe
      probe_timeout: 5s

  - name: anthropic
    type: anthropic
    api_key: "${ANTHROPIC_API_KEY}"
    model_aliases:
      gpt-4o: claude-opus-4-5
      gpt-4o-mini: claude-haiku-3-5
    timeout: 45s
    circuit_breaker:
      error_rate_threshold: 0.5
      consecutive_failures: 5
      window_seconds: 30
      min_requests: 5
      recovery_window: 30s
      probe_timeout: 5s

routing:
  strategy: priority

log:
  level: info
  format: json
```

The `management_listen` setting puts management endpoints on a separate port so they don't interfere with inference traffic (and can be network-restricted separately in production).

</div>

<div class="step">
<div class="step-number">Step 3</div>

## Start infermux and verify both providers are healthy

```bash
infermux serve --config infermux.yml
```

You should see both providers healthy:

```
infermux v0.4.0
providers: openai (healthy), anthropic (healthy)
listening on :8080 (management: :8081)
```

In a second terminal, check the provider status via the management API:

```bash
curl -s http://localhost:8081/_infermux/providers | jq '.[] | {name, healthy, circuit}'
```

```json
{"name": "openai", "healthy": true, "circuit": "closed"}
{"name": "anthropic", "healthy": true, "circuit": "closed"}
```

</div>

<div class="step">
<div class="step-number">Step 4</div>

## Verify that requests go to OpenAI

```bash
curl -s -D - http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4o-mini", "messages": [{"role": "user", "content": "hi"}]}' \
  | grep X-Infermux-Provider
```

```
X-Infermux-Provider: openai
```

Send several requests. They should all go to OpenAI because it is the priority-1 provider and its circuit is closed.

</div>

<div class="step">
<div class="step-number">Step 5</div>

## Simulate an OpenAI outage

To simulate OpenAI going down, manually open its circuit via the management API:

```bash
curl -s -X POST http://localhost:8081/_infermux/providers/openai/circuit/open
```

Verify the circuit is open:

```bash
curl -s http://localhost:8081/_infermux/providers | jq '.[] | {name, circuit}'
```

```json
{"name": "openai", "circuit": "open"}
{"name": "anthropic", "circuit": "closed"}
```

Now send a request:

```bash
curl -s -D - http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4o-mini", "messages": [{"role": "user", "content": "hi"}]}' \
  | grep -E "X-Infermux-Provider|X-Infermux-Model"
```

```
X-Infermux-Provider: anthropic
X-Infermux-Model: claude-haiku-3-5
```

Requests are now routing to Anthropic, which is translating `gpt-4o-mini` to `claude-haiku-3-5` via the model alias. The response body is still in OpenAI format — infermux normalizes it.

</div>

<div class="step">
<div class="step-number">Step 6</div>

## Understand real circuit-breaking (not just manual control)

Manual circuit control is useful for testing and maintenance, but in production the circuit opens automatically. To see this happen, you need to generate real errors against OpenAI.

The easiest way to test this without impacting your account is to temporarily misconfigure the OpenAI API key. Stop infermux, change the config to use a bad key for OpenAI, then restart:

```yaml
providers:
  - name: openai
    type: openai
    api_key: "sk-invalid-key-for-testing"  # will cause 401 errors
    models: [gpt-4o-mini]
    circuit_breaker:
      consecutive_failures: 3    # open faster for testing
      min_requests: 3
```

Start infermux and send requests. After 3 consecutive 401s from OpenAI, the circuit opens automatically and requests shift to Anthropic.

Watch the JSON logs:

```
{"level":"warn","provider":"openai","error":"401 Unauthorized","consecutive_failures":1}
{"level":"warn","provider":"openai","error":"401 Unauthorized","consecutive_failures":2}
{"level":"warn","provider":"openai","error":"401 Unauthorized","consecutive_failures":3}
{"level":"warn","provider":"openai","circuit":"open","reason":"consecutive_failures","msg":"circuit opened"}
{"level":"info","provider":"anthropic","model":"claude-haiku-3-5","latency_ms":312,"msg":"request routed"}
```

After testing, restore the correct API key.

</div>

<div class="step">
<div class="step-number">Step 7</div>

## Verify automatic recovery

With the circuit open manually (from Step 5), wait for the `recovery_window` to expire (20 seconds in your config) and watch the circuit enter half-open, then closed:

```bash
# Poll circuit state every 2 seconds
while true; do
  curl -s http://localhost:8081/_infermux/providers \
    | jq -r '.[] | select(.name=="openai") | "\(.name): \(.circuit)"'
  sleep 2
done
```

You'll see the state progress:

```
openai: open
openai: open
openai: open
openai: open
openai: open
openai: open
openai: open
openai: open
openai: open
openai: half-open    ← recovery window expired, probe sent
openai: closed       ← probe succeeded, back to normal
```

Once the circuit closes, the next request will go back to OpenAI:

```bash
curl -s -D - http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4o-mini", "messages": [{"role": "user", "content": "hi"}]}' \
  | grep X-Infermux-Provider
# X-Infermux-Provider: openai
```

</div>

<div class="step">
<div class="step-number">Step 8</div>

## Check what happens when both providers are down

Open both circuits:

```bash
curl -s -X POST http://localhost:8081/_infermux/providers/openai/circuit/open
curl -s -X POST http://localhost:8081/_infermux/providers/anthropic/circuit/open
```

Send a request:

```bash
curl -s http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4o-mini", "messages": [{"role": "user", "content": "hi"}]}'
```

```json
HTTP/1.1 503 Service Unavailable

{
  "error": {
    "message": "no healthy providers available for model gpt-4o-mini",
    "type": "infermux_error",
    "code": "no_healthy_providers"
  }
}
```

infermux returns a 503. Your application should treat this as a transient failure and retry with backoff, or return a graceful error to the user.

Reset both circuits:

```bash
curl -s -X POST http://localhost:8081/_infermux/providers/openai/circuit/reset
curl -s -X POST http://localhost:8081/_infermux/providers/anthropic/circuit/reset
```

</div>

<div class="callout note">

**Choosing circuit breaker thresholds:** The right thresholds depend on your traffic volume and SLA. A low-traffic service might need `min_requests: 5` so one bad request doesn't trigger the circuit. A high-traffic service can use lower thresholds and a shorter `window_seconds` to detect degradation faster. The `recovery_window` should be long enough for the provider to recover but short enough that you don't miss a full recovery. Start with the defaults and tune based on observed error patterns.

</div>

## What you built

A two-provider infermux setup with automatic failover: OpenAI as primary, Anthropic as fallback. The circuit breaker opens automatically when OpenAI fails, and closes automatically when it recovers. No application code was changed.

## What's next

- [Circuit Breaking](/infermux/docs/circuit-breaking/) — full reference for circuit breaker configuration and the state machine
- [Cost Optimization tutorial](/infermux/tutorials/cost-optimization/) — add cost-weighted routing to minimize spend across your provider set
- [HTTP API](/infermux/docs/http-api/) — full reference for the management API
