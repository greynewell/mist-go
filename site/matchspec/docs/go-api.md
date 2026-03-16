---
title: Go API
description: Use matchspec programmatically from Go — Suite, Harness, Dataset, Model, Grader, and running evals in Go tests.
---

# Go API

The matchspec Go API lets you build eval pipelines entirely in code, without YAML config files. This is useful for co-locating evals with application code, running evals inside Go tests, or building custom eval infrastructure on top of matchspec.

## Install

```bash
go get github.com/greynewell/matchspec
```

## Core types

### Dataset

```go
type Dataset struct {
    Name        string
    Description string
    Examples    []Example
}

type Example struct {
    ID       string
    Input    string
    Expected string
    Tags     []string
    Metadata map[string]any
}
```

Build a dataset inline or load from a file:

```go
// Inline
ds := matchspec.Dataset{
    Name: "qa-basic",
    Examples: []matchspec.Example{
        {ID: "q1", Input: "What is the capital of France?", Expected: "Paris"},
        {ID: "q2", Input: "What is 2 + 2?", Expected: "4"},
    },
}

// From file
ds, err := matchspec.LoadDatasetFile("./evals/qa/dataset.yml")

// From embedded bytes
//go:embed evals/qa/dataset.yml
var dsYAML []byte

ds, err := matchspec.ParseDatasetYAML(dsYAML)
```

### Model

```go
type Model interface {
    Run(ctx context.Context, input string) (string, error)
}
```

The simplest implementation uses `ModelFunc`:

```go
model := matchspec.ModelFunc(func(ctx context.Context, input string) (string, error) {
    return callMyLLM(ctx, input)
})
```

For HTTP models, use the built-in `HTTPModel`:

```go
model := matchspec.NewHTTPModel(matchspec.HTTPModelConfig{
    Endpoint:        "https://api.openai.com/v1/chat/completions",
    APIKey:          os.Getenv("OPENAI_API_KEY"),
    RequestTemplate: `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"{{input}}"}]}`,
    ResponsePath:    "choices[0].message.content",
})
```

### Grader

```go
type Grader interface {
    Name() string
    Score(ctx context.Context, input, expected, output string) (Score, error)
}

type Score struct {
    Value    float64
    Passed   bool
    Metadata map[string]any
}
```

Built-in grader constructors:

```go
// Exact match
g := matchspec.NewExactMatchGrader(matchspec.ExactMatchConfig{
    Threshold:      0.90,
    TrimWhitespace: true,
    CaseSensitive:  true,
})

// Contains
g := matchspec.NewContainsGrader(matchspec.ContainsConfig{
    Threshold:     0.85,
    CaseSensitive: false,
})

// Regex
g := matchspec.NewRegexGrader(matchspec.RegexConfig{
    Pattern:   `^\{.*\}$`,
    Flags:     "s",
    Threshold: 0.95,
})

// Semantic similarity
g := matchspec.NewSemanticSimilarityGrader(matchspec.SemanticSimilarityConfig{
    EmbeddingEndpoint: "https://api.openai.com/v1/embeddings",
    Model:             "text-embedding-3-small",
    APIKey:            os.Getenv("OPENAI_API_KEY"),
    Threshold:         0.82,
    BatchSize:         32,
})

// LLM judge
g := matchspec.NewLLMJudgeGrader(matchspec.LLMJudgeConfig{
    Endpoint:       "https://api.openai.com/v1/chat/completions",
    Model:          "gpt-4o",
    APIKey:         os.Getenv("OPENAI_API_KEY"),
    PromptTemplate: "Score 0-10: {{output}} vs {{expected}}. Reply with only the number.",
    ScoreParser:    matchspec.ScoreParserInt0to10,
    Threshold:      0.75,
})
```

### Harness

```go
type Harness struct {
    Name           string
    Description    string
    Dataset        Dataset
    Model          Model
    Graders        []Grader
    Concurrency    int
    TimeoutSeconds int
    Retries        int
    RetryDelayMs   int
}
```

### Suite

```go
type Suite struct {
    Name       string
    Harnesses  []Harness
    Thresholds Thresholds
    Statistics StatisticsConfig
}

type Thresholds struct {
    Overall     float64
    PerGrader   map[string]float64
}

type StatisticsConfig struct {
    ConfidenceLevel  float64
    UseLowerBound    bool
    MinSampleSize    int
    MinSampleAction  string  // "warn" or "fail"
}
```

## Running a suite

```go
result, err := suite.Run(context.Background())
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Verdict: %s\n", result.Verdict)
for _, gr := range result.GraderResults {
    fmt.Printf("  %s: %.2f (threshold %.2f, passed: %v)\n",
        gr.Name, gr.Score, gr.Threshold, gr.Passed)
}

if !result.Passed() {
    os.Exit(1)
}
```

### SuiteResult

```go
type SuiteResult struct {
    Suite          string
    Verdict        string           // "PASS" or "FAIL"
    GraderResults  []GraderResult
    ExampleResults []ExampleResult
    ModelErrors    int
    StartedAt      time.Time
    FinishedAt     time.Time
    Statistics     StatisticsReport
}

type GraderResult struct {
    Name      string
    Score     float64
    Threshold float64
    Passed    bool
    N         int
    CILower   float64
    CIUpper   float64
}

type ExampleResult struct {
    ID       string
    Input    string
    Expected string
    Output   string
    Scores   map[string]float64
    Passed   bool
    Error    error
}

func (r *SuiteResult) Passed() bool {
    return r.Verdict == "PASS"
}
```

## Running evals in Go tests

matchspec integrates cleanly with `go test`. Use it to assert that model quality hasn't regressed as part of your test suite:

```go
package evals_test

import (
    "context"
    "os"
    "testing"

    "github.com/greynewell/matchspec"
)

func TestSummarizationQuality(t *testing.T) {
    if os.Getenv("OPENAI_API_KEY") == "" {
        t.Skip("OPENAI_API_KEY not set; skipping LLM eval")
    }

    ds := matchspec.Dataset{
        Name: "summarization-basic",
        Examples: []matchspec.Example{
            {
                ID:       "s1",
                Input:    "Summarize: Structured pruning reduces neural net training compute by 40%.",
                Expected: "Structured pruning reduces training compute by 40%.",
            },
            {
                ID:       "s2",
                Input:    "Summarize: The council voted 7-2 to allow 8-story buildings downtown.",
                Expected: "The council approved taller buildings downtown.",
            },
        },
    }

    model := matchspec.NewHTTPModel(matchspec.HTTPModelConfig{
        Endpoint:        "https://api.openai.com/v1/chat/completions",
        APIKey:          os.Getenv("OPENAI_API_KEY"),
        RequestTemplate: `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"{{input}}"}]}`,
        ResponsePath:    "choices[0].message.content",
    })

    grader := matchspec.NewSemanticSimilarityGrader(matchspec.SemanticSimilarityConfig{
        EmbeddingEndpoint: "https://api.openai.com/v1/embeddings",
        Model:             "text-embedding-3-small",
        APIKey:            os.Getenv("OPENAI_API_KEY"),
        Threshold:         0.82,
    })

    suite := matchspec.Suite{
        Name: "summarization-test",
        Harnesses: []matchspec.Harness{
            {
                Name:    "summarization-v1",
                Dataset: ds,
                Model:   model,
                Graders: []matchspec.Grader{grader},
            },
        },
        Thresholds: matchspec.Thresholds{Overall: 0.80},
    }

    result, err := suite.Run(context.Background())
    if err != nil {
        t.Fatalf("suite run failed: %v", err)
    }

    for _, gr := range result.GraderResults {
        if !gr.Passed {
            t.Errorf("grader %q failed: score %.2f < threshold %.2f",
                gr.Name, gr.Score, gr.Threshold)
        }
    }

    // Log per-example results for debugging.
    for _, ex := range result.ExampleResults {
        if !ex.Passed {
            t.Logf("FAIL example %s: output=%q", ex.ID, ex.Output)
        }
    }
}
```

Run the eval tests with the standard `go test` command:

```bash
go test ./evals/... -v -timeout 120s
```

Use build tags to separate eval tests from unit tests:

```go
//go:build eval

package evals_test
```

```bash
# Run only unit tests (fast, no LLM calls)
go test ./...

# Run eval tests too
go test -tags eval ./...
```

## Loading a suite from YAML config

You can load a suite from your `matchspec.yml` file and run it from Go code:

```go
cfg, err := matchspec.LoadConfig("./matchspec.yml")
if err != nil {
    log.Fatal(err)
}

suite, err := cfg.Suite("production-gate")
if err != nil {
    log.Fatal(err)
}

result, err := suite.Run(context.Background())
if err != nil {
    log.Fatal(err)
}

if !result.Passed() {
    log.Fatalf("suite failed: %s", result.Summary())
}
```

## Writing results to JSON

```go
result, err := suite.Run(context.Background())
if err != nil {
    log.Fatal(err)
}

if err := matchspec.WriteResultsJSON(result, "./results.json"); err != nil {
    log.Fatal(err)
}
```

The JSON format matches the schema returned by the [HTTP API](/matchspec/docs/http-api/).
