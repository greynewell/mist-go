---
title: "Route your first inference request"
description: "Install infermux, configure one provider, and send your first routed inference request in 10 minutes."
difficulty: beginner
duration: "10 min"
---

# Route your first inference request

<div class="tutorial-meta">
  <span class="meta-tag">beginner</span>
  <span class="meta-tag">10 min</span>
</div>

In this tutorial you will install infermux, write a config file with a single OpenAI provider, start the server, and send a request through the router. By the end you'll understand how infermux works as a transparent proxy and what information it adds to each response.

**What you need:**
- Go 1.21 or later
- An OpenAI API key (get one at [platform.openai.com](https://platform.openai.com))
- `curl` for testing

---

<div class="step">
<div class="step-number">Step 1</div>

## Install infermux

Install the CLI with `go install`:

```bash
go install github.com/greynewell/infermux/cmd/infermux@latest
```

This compiles infermux and places the binary at `$(go env GOPATH)/bin/infermux`. Add it to your path if it isn't already:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
infermux --version
# infermux v0.4.0
```

</div>

<div class="step">
<div class="step-number">Step 2</div>

## Set your API key

```bash
export OPENAI_API_KEY=sk-proj-...
```

infermux reads environment variables at startup. It never writes them to disk or logs them.

</div>

<div class="step">
<div class="step-number">Step 3</div>

## Write a config file

Create a file called `infermux.yml` in your working directory:

```yaml
listen: ":8080"

providers:
  - name: openai
    type: openai
    api_key: "${OPENAI_API_KEY}"
    models:
      - gpt-4o-mini
      - gpt-4o

routing:
  strategy: round_robin
```

This config declares one provider (OpenAI), lists the models it can serve, and sets the routing strategy to round-robin. With one provider, round-robin always routes to that provider — it's effectively a transparent proxy at this point.

</div>

<div class="step">
<div class="step-number">Step 4</div>

## Start the server

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

The `(healthy)` status means infermux successfully ran a startup health check against the OpenAI API. If you see `(unhealthy)`, check your `OPENAI_API_KEY`.

Leave this terminal running. Open a new terminal for the next step.

</div>

<div class="step">
<div class="step-number">Step 5</div>

## Send your first request

infermux exposes the OpenAI-compatible `/v1/chat/completions` endpoint. Send a request the same way you'd send it to OpenAI directly — just change the host:

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

You should receive a standard OpenAI response:

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
    "prompt_tokens": 15,
    "completion_tokens": 2,
    "total_tokens": 17
  }
}
```

</div>

<div class="step">
<div class="step-number">Step 6</div>

## Inspect the routing headers

Add `-D -` to curl to see the response headers:

```bash
curl -D - http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4o-mini", "messages": [{"role": "user", "content": "hello"}]}' \
  2>/dev/null | head -20
```

Look for the infermux-specific headers:

```
HTTP/1.1 200 OK
Content-Type: application/json
X-Infermux-Provider: openai
X-Infermux-Model: gpt-4o-mini
X-Infermux-Strategy: round_robin
X-Infermux-Latency-Ms: 387
X-Infermux-Cost: 0.0000018
X-Infermux-Prompt-Tokens: 15
X-Infermux-Completion-Tokens: 2
```

These headers tell you:
- Which provider served the request (`openai`)
- The routing strategy used (`round_robin`)
- How long the provider took to respond (`387ms`)
- The estimated cost in USD (`$0.0000018`)

</div>

<div class="step">
<div class="step-number">Step 7</div>

## Use infermux from Go

If your application uses the Go OpenAI client, change the base URL:

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/openai/openai-go"
    "github.com/openai/openai-go/option"
)

func main() {
    // Point the client at infermux instead of api.openai.com
    client := openai.NewClient(
        option.WithBaseURL("http://localhost:8080/v1"),
        option.WithAPIKey("any-value"), // infermux uses its own credentials
    )

    chat, err := client.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
        Model: openai.F(openai.ChatModelGPT4oMini),
        Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
            openai.UserMessage("What is the capital of France?"),
        }),
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(chat.Choices[0].Message.Content)
}
```

This is the full drop-in replacement: one line changes (`WithBaseURL`) and your application gains routing, failover, and cost tracking.

</div>

<div class="callout note">

**What you built:** A running infermux instance routing requests to OpenAI. The response is identical to calling OpenAI directly — infermux adds routing metadata in headers but does not modify the response body.

</div>

## What's next

- [Quick Start](/infermux/docs/quick-start/) — add a second provider and watch round-robin routing
- [Failover Setup tutorial](/infermux/tutorials/failover-setup/) — configure automatic failover between OpenAI and Anthropic
- [Circuit Breaking](/infermux/docs/circuit-breaking/) — understand the circuit breaker thresholds and recovery behavior
