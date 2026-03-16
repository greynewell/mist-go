---
title: Custom Graders
description: Implement the Grader interface, handle HTTP clients and LLM APIs, write stateful graders, and register custom graders for YAML config.
---

# Custom Graders

When the built-in graders don't capture what "correct" means for your use case, you can implement the `Grader` interface. Custom graders are regular Go code — they can call APIs, use embedding models, apply domain rules, or do anything else that returns a score.

## The Grader interface

```go
type Grader interface {
    Name() string
    Score(ctx context.Context, input, expected, output string) (Score, error)
}

type Score struct {
    Value    float64        // 0.0 to 1.0
    Passed   bool           // true if Value >= threshold
    Metadata map[string]any // optional debug info attached to per-example results
}
```

`Score` must return a value between 0.0 and 1.0. matchspec does not clamp the value, so values outside this range will produce incorrect results. Return an error only for unexpected failures (network errors, panics, malformed data) — a low score is not an error.

## Grader lifecycle

matchspec calls `Score` once per example, with the context that was passed to `Suite.Run`. The context carries a deadline based on the harness `timeout_seconds` setting. Honor it:

```go
func (g *MyGrader) Score(ctx context.Context, input, expected, output string) (matchspec.Score, error) {
    req, err := http.NewRequestWithContext(ctx, "POST", g.endpoint, body)
    // ...
}
```

If `Score` returns an error, the example is marked as a grader error and excluded from the pass rate calculation. Grader errors are reported separately from model errors.

## Minimal example

```go
package graders

import (
    "context"
    "strings"

    "github.com/greynewell/matchspec"
)

// LengthRatioGrader scores by how close the output length is to the expected length.
// Score = 1.0 - |len(output) - len(expected)| / len(expected)
// Clamped to [0.0, 1.0].
type LengthRatioGrader struct {
    threshold float64
}

func NewLengthRatioGrader(threshold float64) *LengthRatioGrader {
    return &LengthRatioGrader{threshold: threshold}
}

func (g *LengthRatioGrader) Name() string { return "length_ratio" }

func (g *LengthRatioGrader) Score(_ context.Context, _, expected, output string) (matchspec.Score, error) {
    exp := len(strings.TrimSpace(expected))
    got := len(strings.TrimSpace(output))

    if exp == 0 {
        // Degenerate case: empty expected. Score 1.0 if output is also empty.
        v := 0.0
        if got == 0 {
            v = 1.0
        }
        return matchspec.Score{Value: v, Passed: v >= g.threshold}, nil
    }

    diff := float64(abs(got-exp)) / float64(exp)
    score := max(0.0, 1.0-diff)

    return matchspec.Score{
        Value:  score,
        Passed: score >= g.threshold,
        Metadata: map[string]any{
            "expected_len": exp,
            "output_len":   got,
        },
    }, nil
}

func abs(n int) int {
    if n < 0 {
        return -n
    }
    return n
}

func max(a, b float64) float64 {
    if a > b {
        return a
    }
    return b
}
```

## Accessing HTTP clients and LLM APIs

For graders that call external services, inject an `*http.Client` or a custom client at construction time:

```go
type JSONSchemaGrader struct {
    schemaEndpoint string
    client         *http.Client
    threshold      float64
}

func NewJSONSchemaGrader(endpoint string, threshold float64) *JSONSchemaGrader {
    return &JSONSchemaGrader{
        schemaEndpoint: endpoint,
        client:         &http.Client{Timeout: 10 * time.Second},
        threshold:      threshold,
    }
}

func (g *JSONSchemaGrader) Name() string { return "json_schema" }

func (g *JSONSchemaGrader) Score(ctx context.Context, _, expected, output string) (matchspec.Score, error) {
    // Validate output against a JSON schema served at schemaEndpoint.
    body := strings.NewReader(fmt.Sprintf(`{"schema":%s,"instance":%s}`, expected, output))
    req, err := http.NewRequestWithContext(ctx, "POST", g.schemaEndpoint+"/validate", body)
    if err != nil {
        return matchspec.Score{}, err
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := g.client.Do(req)
    if err != nil {
        return matchspec.Score{}, fmt.Errorf("schema validation request failed: %w", err)
    }
    defer resp.Body.Close()

    var result struct {
        Valid   bool     `json:"valid"`
        Errors  []string `json:"errors"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return matchspec.Score{}, fmt.Errorf("failed to decode validation response: %w", err)
    }

    score := 0.0
    if result.Valid {
        score = 1.0
    }

    return matchspec.Score{
        Value:  score,
        Passed: score >= g.threshold,
        Metadata: map[string]any{
            "valid":  result.Valid,
            "errors": result.Errors,
        },
    }, nil
}
```

## Stateful graders

Some graders need to accumulate state across multiple examples — for example, a grader that batches embedding requests for efficiency, or one that tracks a calibration baseline.

Implement the optional `matchspec.GraderWithSetup` interface:

```go
type GraderWithSetup interface {
    Grader
    Setup(ctx context.Context) error
    Teardown(ctx context.Context) error
}
```

matchspec calls `Setup` before the first example and `Teardown` after the last. Use `Setup` to initialize connections or load models, and `Teardown` to flush buffers and close connections.

```go
type BatchEmbeddingGrader struct {
    endpoint  string
    apiKey    string
    threshold float64
    mu        sync.Mutex
    batch     []batchItem
    results   map[string]float64
}

type batchItem struct {
    id       string
    expected string
    output   string
    done     chan struct{}
}

func (g *BatchEmbeddingGrader) Name() string { return "batch_embedding" }

func (g *BatchEmbeddingGrader) Setup(ctx context.Context) error {
    // Start background batch processor.
    go g.processBatches(ctx)
    return nil
}

func (g *BatchEmbeddingGrader) Teardown(ctx context.Context) error {
    // Flush any pending batches.
    return g.flush(ctx)
}

func (g *BatchEmbeddingGrader) Score(ctx context.Context, _, expected, output string) (matchspec.Score, error) {
    // Enqueue this example and wait for the batch to complete.
    item := batchItem{
        id:       uuid.New().String(),
        expected: expected,
        output:   output,
        done:     make(chan struct{}),
    }
    g.mu.Lock()
    g.batch = append(g.batch, item)
    g.mu.Unlock()

    select {
    case <-item.done:
        score := g.results[item.id]
        return matchspec.Score{Value: score, Passed: score >= g.threshold}, nil
    case <-ctx.Done():
        return matchspec.Score{}, ctx.Err()
    }
}
```

## Testing graders

Test graders in isolation before wiring them into a suite:

```go
package graders_test

import (
    "context"
    "testing"

    "github.com/greynewell/matchspec"
    "myapp/graders"
)

func TestWordOverlapGrader(t *testing.T) {
    g := graders.NewWordOverlapGrader(0.70)

    tests := []struct {
        name     string
        expected string
        output   string
        wantMin  float64
        wantMax  float64
    }{
        {
            name:     "exact match",
            expected: "the quick brown fox",
            output:   "the quick brown fox",
            wantMin:  1.0,
            wantMax:  1.0,
        },
        {
            name:     "half overlap",
            expected: "the quick brown fox",
            output:   "the quick red dog",
            wantMin:  0.45,
            wantMax:  0.55,
        },
        {
            name:     "no overlap",
            expected: "paris is the capital of france",
            output:   "2 + 2 = 4",
            wantMin:  0.0,
            wantMax:  0.10,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            score, err := g.Score(context.Background(), "", tt.expected, tt.output)
            if err != nil {
                t.Fatalf("unexpected error: %v", err)
            }
            if score.Value < tt.wantMin || score.Value > tt.wantMax {
                t.Errorf("Score() = %.3f, want [%.3f, %.3f]",
                    score.Value, tt.wantMin, tt.wantMax)
            }
        })
    }
}
```

Test the full grader in a harness with a stub model to verify end-to-end behavior:

```go
func TestWordOverlapGraderInHarness(t *testing.T) {
    ds := matchspec.Dataset{
        Name: "test",
        Examples: []matchspec.Example{
            {ID: "1", Input: "x", Expected: "the quick brown fox"},
            {ID: "2", Input: "x", Expected: "paris is the capital"},
        },
    }

    // Stub model returns a known output.
    model := matchspec.ModelFunc(func(_ context.Context, _ string) (string, error) {
        return "the quick brown dog", nil
    })

    grader := graders.NewWordOverlapGrader(0.70)

    suite := matchspec.Suite{
        Name: "test",
        Harnesses: []matchspec.Harness{{
            Name:    "test",
            Dataset: ds,
            Model:   model,
            Graders: []matchspec.Grader{grader},
        }},
        Thresholds: matchspec.Thresholds{Overall: 0.50},
    }

    result, err := suite.Run(context.Background())
    if err != nil {
        t.Fatal(err)
    }

    t.Logf("word_overlap score: %.3f", result.GraderResults[0].Score)
}
```

## Registering custom graders

To use a custom grader by name in YAML harness configs, register it with `matchspec.RegisterGrader`:

```go
package main

import (
    "github.com/greynewell/matchspec"
    "myapp/graders"
)

func init() {
    matchspec.RegisterGrader("word_overlap", func(cfg map[string]any) (matchspec.Grader, error) {
        threshold := 0.70
        if v, ok := cfg["threshold"].(float64); ok {
            threshold = v
        }
        return graders.NewWordOverlapGrader(threshold), nil
    })

    matchspec.RegisterGrader("json_schema", func(cfg map[string]any) (matchspec.Grader, error) {
        endpoint, _ := cfg["endpoint"].(string)
        threshold := 0.90
        if v, ok := cfg["threshold"].(float64); ok {
            threshold = v
        }
        if endpoint == "" {
            return nil, fmt.Errorf("json_schema grader requires 'endpoint' config")
        }
        return graders.NewJSONSchemaGrader(endpoint, threshold), nil
    })
}
```

The `init()` function runs before `matchspec.LoadConfig` processes any harness files. If you're using the CLI, register graders in a Go plugin or in the main package of a custom binary that wraps `matchspec.Run`.

After registration, reference the grader type in YAML:

```yaml
graders:
  - type: word_overlap
    name: word_overlap
    threshold: 0.72
    config:
      threshold: 0.72

  - type: json_schema
    name: output_format
    threshold: 0.95
    config:
      endpoint: "http://localhost:9000"
      threshold: 0.95
```
