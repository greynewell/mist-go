---
title: "Go API"
description: "Use infermux as a Go library. Create a Router, register providers, make requests, subscribe to metrics, and test with mock providers."
---

# Go API

infermux can be embedded in a Go program as a library. This is useful when you want the routing and circuit breaking logic in-process rather than as a separate server, or when you need to integrate tightly with a Go application's lifecycle, metrics, and testing infrastructure.

## Import

```go
import "github.com/greynewell/infermux"
```

## Creating a Router

The `Router` is the central type. Create one from a `Config`:

```go
cfg := infermux.Config{
    Providers: []infermux.ProviderConfig{
        {
            Name:   "openai",
            Type:   infermux.ProviderOpenAI,
            APIKey: os.Getenv("OPENAI_API_KEY"),
            Models: []string{"gpt-4o", "gpt-4o-mini"},
        },
        {
            Name:   "anthropic",
            Type:   infermux.ProviderAnthropic,
            APIKey: os.Getenv("ANTHROPIC_API_KEY"),
            ModelAliases: map[string]string{
                "gpt-4o":      "claude-opus-4-5",
                "gpt-4o-mini": "claude-haiku-3-5",
            },
        },
    },
    Routing: infermux.RoutingConfig{
        Strategy: infermux.StrategyPriority,
    },
    CircuitBreaker: infermux.CircuitBreakerConfig{
        ErrorRateThreshold:  0.5,
        ConsecutiveFailures: 5,
        RecoveryWindow:      30 * time.Second,
    },
}

router, err := infermux.New(cfg)
if err != nil {
    log.Fatal(err)
}
defer router.Close()
```

## Making requests

The `Router` implements `http.Handler`. You can use it directly as an HTTP handler:

```go
http.ListenAndServe(":8080", router)
```

You can also make requests programmatically using the Go client interface:

```go
req := &infermux.ChatRequest{
    Model: "gpt-4o-mini",
    Messages: []infermux.Message{
        {Role: "user", Content: "What is the capital of France?"},
    },
    MaxTokens: 64,
}

resp, err := router.Chat(ctx, req)
if err != nil {
    return fmt.Errorf("chat request failed: %w", err)
}

fmt.Println(resp.Choices[0].Message.Content)
fmt.Printf("provider: %s, cost: $%.6f\n", resp.Meta.Provider, resp.Meta.CostUSD)
```

The `ChatResponse.Meta` field is infermux-specific metadata that is not part of the OpenAI response schema:

```go
type ResponseMeta struct {
    Provider         string
    Model            string
    Strategy         string
    RouteGroup       string
    LatencyMs        int64
    CostUSD          float64
    PromptTokens     int
    CompletionTokens int
}
```

## Registering providers dynamically

Providers can be added and removed at runtime without creating a new Router:

```go
err := router.AddProvider(infermux.ProviderConfig{
    Name:    "groq",
    Type:    infermux.ProviderOpenAI,
    APIKey:  os.Getenv("GROQ_API_KEY"),
    BaseURL: "https://api.groq.com/openai/v1",
    Models:  []string{"llama-3.1-70b-versatile"},
})
if err != nil {
    return err
}

// Remove a provider (waits for in-flight requests to complete)
err = router.RemoveProvider("groq")
```

## Context and attribution

Attribution fields are passed via context:

```go
ctx = infermux.WithAttribution(ctx, infermux.Attribution{
    Caller:  "user-abc123",
    Project: "summarization",
    Env:     "production",
})

resp, err := router.Chat(ctx, req)
```

## Subscribing to metrics events

infermux emits events on the mist-go metrics bus. Subscribe to receive cost and latency data for every request:

```go
import "github.com/greynewell/mist-go/metrics"

router.OnRequest(func(e infermux.RequestEvent) {
    log.Printf(
        "provider=%s model=%s latency=%dms cost=$%.6f tokens_in=%d tokens_out=%d",
        e.Provider, e.Model, e.LatencyMs, e.CostUSD,
        e.PromptTokens, e.CompletionTokens,
    )
})

router.OnCircuitStateChange(func(provider string, from, to infermux.CircuitState) {
    log.Printf("circuit %s: %s -> %s", provider, from, to)
    // trigger your alerting system here
})
```

Events are emitted synchronously in the request goroutine. Keep event handlers fast; do async work in a goroutine if needed.

## Embeddings

```go
req := &infermux.EmbeddingRequest{
    Model: "text-embedding-3-small",
    Input: []string{"The quick brown fox", "Pack my box with five dozen liquor jugs"},
}

resp, err := router.Embed(ctx, req)
if err != nil {
    return err
}

fmt.Printf("embedding dimensions: %d\n", len(resp.Data[0].Embedding))
```

## Inspecting circuit state

```go
states := router.CircuitStates()
for name, state := range states {
    fmt.Printf("%s: %s\n", name, state)
}

// Manually open/close a circuit
router.OpenCircuit("openai")
router.CloseCircuit("openai")
```

## Testing with a mock provider

infermux ships a `mockprovider` package that implements the provider interface with configurable responses. Use it in tests to verify routing logic, circuit breaking behavior, and cost attribution without making real API calls.

```go
import "github.com/greynewell/infermux/mockprovider"

func TestFailover(t *testing.T) {
    primary := mockprovider.New("primary")
    primary.SetModels([]string{"gpt-4o"})
    primary.SetResponse(&infermux.ChatResponse{
        Choices: []infermux.Choice{
            {Message: infermux.Message{Role: "assistant", Content: "hello from primary"}},
        },
    })

    fallback := mockprovider.New("fallback")
    fallback.SetModels([]string{"gpt-4o"})
    fallback.SetResponse(&infermux.ChatResponse{
        Choices: []infermux.Choice{
            {Message: infermux.Message{Role: "assistant", Content: "hello from fallback"}},
        },
    })

    router, _ := infermux.NewWithProviders(
        []infermux.Provider{primary, fallback},
        infermux.RoutingConfig{Strategy: infermux.StrategyPriority},
        infermux.CircuitBreakerConfig{ConsecutiveFailures: 2},
    )

    ctx := context.Background()
    req := &infermux.ChatRequest{
        Model:    "gpt-4o",
        Messages: []infermux.Message{{Role: "user", Content: "hi"}},
    }

    // First two requests succeed on primary
    for i := 0; i < 2; i++ {
        resp, err := router.Chat(ctx, req)
        require.NoError(t, err)
        assert.Equal(t, "hello from primary", resp.Choices[0].Message.Content)
    }

    // Make primary fail
    primary.SetError(errors.New("connection refused"))

    // After ConsecutiveFailures=2, primary circuit opens, fallback takes over
    for i := 0; i < 4; i++ {
        resp, err := router.Chat(ctx, req)
        require.NoError(t, err)
        if i < 2 {
            // circuit not yet open
            assert.Equal(t, "primary", resp.Meta.Provider)
        } else {
            assert.Equal(t, "fallback", resp.Meta.Provider)
        }
    }

    assert.Equal(t, "open", string(router.CircuitStates()["primary"]))
}
```

The mock provider also supports injecting latency, simulating rate limits, and tracking which requests were received — useful for testing that your routing configuration sends the right traffic to the right provider.

```go
primary.SetLatency(200 * time.Millisecond)
primary.SetRateLimit(10) // fail with 429 after 10 requests

received := primary.Requests()
assert.Equal(t, "gpt-4o", received[0].Model)
```
