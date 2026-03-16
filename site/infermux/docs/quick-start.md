---
title: "Quick Start"
description: "Route your first inference request through infermux in five minutes."
---

# Quick Start

This guide gets infermux running with a single provider and shows you a working request. It then adds a second provider so you can see routing in action. You'll need an OpenAI API key for the first part; an Anthropic key for the second.

## 1. Install

```bash
go install github.com/greynewell/infermux/cmd/infermux@latest
```

Verify the installation:

```bash
infermux --version
# infermux v0.4.0
```

## 2. Write a minimal config

Create `infermux.yml` in your working directory:

```yaml
listen: ":8080"

providers:
  - name: openai
    type: openai
    api_key: "${OPENAI_API_KEY}"
    models:
      - gpt-4o
      - gpt-4o-mini
      - gpt-3.5-turbo

routing:
  strategy: round_robin
```

Export your API key:

```bash
export OPENAI_API_KEY=sk-...
```

## 3. Start the server

```bash
infermux serve --config infermux.yml
```

You should see:

```
infermux v0.4.0
config: infermux.yml
providers: openai (healthy)
listening on :8080
```

## 4. Make a request

infermux exposes the standard OpenAI `/v1/chat/completions` endpoint. Point any OpenAI client at `http://localhost:8080` instead of `https://api.openai.com`:

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [
      {"role": "user", "content": "What is the capital of France?"}
    ]
  }'
```

The response is the standard OpenAI response format. infermux adds three headers:

```
X-Infermux-Provider: openai
X-Infermux-Model: gpt-4o-mini
X-Infermux-Cost: 0.000018
```

Using the Go OpenAI client is just a base URL change:

```go
import "github.com/openai/openai-go"
import "github.com/openai/openai-go/option"

client := openai.NewClient(
    option.WithBaseURL("http://localhost:8080/v1"),
    option.WithAPIKey("any-value"), // infermux handles auth to providers
)

chat, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
    Model: openai.F(openai.ChatModelGPT4oMini),
    Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
        openai.UserMessage("What is the capital of France?"),
    }),
})
```

## 5. Add a second provider and see routing

Update `infermux.yml` to add Anthropic:

```yaml
listen: ":8080"

providers:
  - name: openai
    type: openai
    api_key: "${OPENAI_API_KEY}"
    models:
      - gpt-4o
      - gpt-4o-mini

  - name: anthropic
    type: anthropic
    api_key: "${ANTHROPIC_API_KEY}"
    model_aliases:
      gpt-4o: claude-opus-4-5
      gpt-4o-mini: claude-haiku-3-5

routing:
  strategy: round_robin
```

Restart infermux:

```bash
infermux serve --config infermux.yml
# providers: openai (healthy), anthropic (healthy)
# listening on :8080
```

Send two requests and watch the `X-Infermux-Provider` header alternate:

```bash
for i in 1 2; do
  curl -s -D - http://localhost:8080/v1/chat/completions \
    -H "Content-Type: application/json" \
    -d '{"model": "gpt-4o-mini", "messages": [{"role": "user", "content": "hi"}]}' \
    | grep X-Infermux-Provider
done
# X-Infermux-Provider: openai
# X-Infermux-Provider: anthropic
```

The `model_aliases` field on the Anthropic provider tells infermux that when a request asks for `gpt-4o-mini`, it should send `claude-haiku-3-5` to Anthropic. The response comes back with the model name normalized to whatever the caller requested.

## What's next

- [Providers](/infermux/docs/providers/) — configure all supported providers and add custom ones
- [Routing](/infermux/docs/routing/) — round-robin, least-latency, cost-weighted, and priority strategies
- [Circuit Breaking](/infermux/docs/circuit-breaking/) — automatic failover and recovery
- [Cost Tracking](/infermux/docs/cost-tracking/) — per-request attribution and budget enforcement
