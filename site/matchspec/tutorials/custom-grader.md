---
layout: tutorial-layout.njk
tool: matchspec
title: Write a Custom Grader
description: Implement the Grader interface to score model outputs against domain-specific rules, test it in isolation, and register it for use in YAML config.
difficulty: intermediate
time: "15 minutes"
---

# Write a Custom Grader

<div class="tutorial-meta">
  <span class="meta-tag">intermediate</span>
  <span>15 minutes</span>
</div>

The built-in graders handle most eval cases, but sometimes "correct" means something domain-specific: structured output validates against a schema, a response contains required fields, or a summary hits the right length bracket. This tutorial shows how to implement the `Grader` interface, test it, and wire it into a harness.

## Prerequisites

- Go 1.21 or later
- A matchspec project (see [Run Your First Eval Suite](/matchspec/tutorials/first-eval/))

---

<div class="step">
<div class="step-number">Step 1</div>

## Understand what we're building

We'll write a `KeywordPresenceGrader` that scores a model output by what fraction of required keywords (taken from the expected output) appear in the actual output. This is useful for summarization tasks where you want to verify that key entities and numbers are preserved.

For example:
- Expected: `"Researchers cut training compute 40% using structured pruning."`
- Output: `"Scientists reduced compute requirements by 40%."`
- Keywords extracted: `["researchers", "training", "compute", "40%", "structured", "pruning"]`
- Matched in output: `["compute", "40%", "structured"]` — 3 of 6 → score 0.50

</div>

<div class="step">
<div class="step-number">Step 2</div>

## Create the grader package

```bash
mkdir -p graders
```

Create `graders/keyword_presence.go`:

```go
package graders

import (
    "context"
    "strings"
    "unicode"

    "github.com/greynewell/matchspec"
)

// KeywordPresenceGrader scores the fraction of keywords from the expected
// output that appear in the model output. Keywords are extracted by
// tokenizing the expected string and filtering to words of length >= minLen.
type KeywordPresenceGrader struct {
    threshold float64
    minLen    int
}

func NewKeywordPresenceGrader(threshold float64, minLen int) *KeywordPresenceGrader {
    if minLen <= 0 {
        minLen = 4
    }
    return &KeywordPresenceGrader{threshold: threshold, minLen: minLen}
}

func (g *KeywordPresenceGrader) Name() string { return "keyword_presence" }

func (g *KeywordPresenceGrader) Score(
    _ context.Context,
    _ string,
    expected string,
    output string,
) (matchspec.Score, error) {
    keywords := g.extractKeywords(expected)
    if len(keywords) == 0 {
        return matchspec.Score{
            Value:    1.0,
            Passed:   true,
            Metadata: map[string]any{"keywords": []string{}, "matched": 0},
        }, nil
    }

    outputLower := strings.ToLower(output)
    var matched, missing []string

    for _, kw := range keywords {
        if strings.Contains(outputLower, kw) {
            matched = append(matched, kw)
        } else {
            missing = append(missing, kw)
        }
    }

    score := float64(len(matched)) / float64(len(keywords))

    return matchspec.Score{
        Value:  score,
        Passed: score >= g.threshold,
        Metadata: map[string]any{
            "keywords_total":   len(keywords),
            "keywords_matched": len(matched),
            "matched":          matched,
            "missing":          missing,
        },
    }, nil
}

func (g *KeywordPresenceGrader) extractKeywords(s string) []string {
    words := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
        return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '%'
    })

    seen := make(map[string]bool)
    var keywords []string
    for _, w := range words {
        w = strings.Trim(w, ".,;:!?")
        if len(w) >= g.minLen && !seen[w] {
            seen[w] = true
            keywords = append(keywords, w)
        }
    }
    return keywords
}
```

The `Score` method:
1. Extracts keywords from `expected`
2. Checks which appear in `output`
3. Returns a score from 0.0 to 1.0
4. Attaches debug metadata — visible in the JSON results file

</div>

<div class="step">
<div class="step-number">Step 3</div>

## Test the grader in isolation

Create `graders/keyword_presence_test.go`:

```go
package graders_test

import (
    "context"
    "testing"

    "myapp/graders"
)

func TestKeywordPresenceGrader(t *testing.T) {
    g := graders.NewKeywordPresenceGrader(0.70, 4)

    tests := []struct {
        name     string
        expected string
        output   string
        wantMin  float64
        wantMax  float64
        wantPass bool
    }{
        {
            name:     "all keywords present",
            expected: "Researchers cut training compute 40% using structured pruning.",
            output:   "Scientists reduced compute by 40% via structured pruning techniques.",
            wantMin:  0.60,
            wantMax:  1.01,
            wantPass: true,
        },
        {
            name:     "no keywords present",
            expected: "Researchers cut training compute 40% using structured pruning.",
            output:   "Scientists published a paper.",
            wantMin:  0.0,
            wantMax:  0.30,
            wantPass: false,
        },
        {
            name:     "empty expected gives full credit",
            expected: "",
            output:   "anything",
            wantMin:  1.0,
            wantMax:  1.0,
            wantPass: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            score, err := g.Score(context.Background(), "", tt.expected, tt.output)
            if err != nil {
                t.Fatalf("unexpected error: %v", err)
            }
            if score.Value < tt.wantMin || score.Value > tt.wantMax {
                t.Errorf("Score()=%.3f, want [%.3f, %.3f]", score.Value, tt.wantMin, tt.wantMax)
            }
            if score.Passed != tt.wantPass {
                t.Errorf("Passed()=%v, want %v (score=%.3f)", score.Passed, tt.wantPass, score.Value)
            }
        })
    }
}
```

Run the tests:

```bash
go test ./graders/... -v
```

```
--- PASS: TestKeywordPresenceGrader/all_keywords_present (0.00s)
--- PASS: TestKeywordPresenceGrader/no_keywords_present (0.00s)
--- PASS: TestKeywordPresenceGrader/empty_expected_gives_full_credit (0.00s)
PASS
ok  	myapp/graders	0.012s
```

Test your grader thoroughly before wiring it into a suite. A grader with a logic error will silently produce wrong pass rates.

</div>

<div class="step">
<div class="step-number">Step 4</div>

## Use the grader in a harness (Go API)

Wire the grader into a harness using the Go API:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/greynewell/matchspec"
    "myapp/graders"
)

func main() {
    ds := matchspec.Dataset{
        Name: "summarization-keywords",
        Examples: []matchspec.Example{
            {
                ID:       "s1",
                Input:    "Summarize: MIT reduced neural net training compute by 40% with structured pruning.",
                Expected: "MIT reduced training compute 40% using structured pruning.",
            },
            {
                ID:       "s2",
                Input:    "Summarize: The council voted 7-2 to allow 8-story buildings downtown.",
                Expected: "Council approved 8-story downtown buildings in a 7-2 vote.",
            },
        },
    }

    model := matchspec.NewHTTPModel(matchspec.HTTPModelConfig{
        Endpoint:        "https://api.openai.com/v1/chat/completions",
        APIKey:          os.Getenv("OPENAI_API_KEY"),
        RequestTemplate: `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"{{input}}"}]}`,
        ResponsePath:    "choices[0].message.content",
    })

    kpGrader := graders.NewKeywordPresenceGrader(0.70, 4)

    suite := matchspec.Suite{
        Name: "summarization-kp",
        Harnesses: []matchspec.Harness{
            {
                Name:    "summarization-v1",
                Dataset: ds,
                Model:   model,
                Graders: []matchspec.Grader{kpGrader},
            },
        },
        Thresholds: matchspec.Thresholds{Overall: 0.70},
    }

    result, err := suite.Run(context.Background())
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Verdict: %s\n", result.Verdict)
    for _, gr := range result.GraderResults {
        fmt.Printf("  %s: %.2f\n", gr.Name, gr.Score)
    }

    // Print metadata from each example to see which keywords were missing.
    for _, ex := range result.ExampleResults {
        if !ex.Passed {
            fmt.Printf("  FAIL %s: missing=%v\n",
                ex.ID, ex.Metadata["keyword_presence.missing"])
        }
    }

    if !result.Passed() {
        os.Exit(1)
    }
}
```

</div>

<div class="step">
<div class="step-number">Step 5</div>

## Register the grader for YAML config

To use your grader by name in YAML harness files, register it at startup:

Create `graders/register.go`:

```go
package graders

import (
    "fmt"

    "github.com/greynewell/matchspec"
)

func init() {
    matchspec.RegisterGrader("keyword_presence", func(cfg map[string]any) (matchspec.Grader, error) {
        threshold := 0.70
        if v, ok := cfg["threshold"].(float64); ok {
            threshold = v
        }

        minLen := 4
        if v, ok := cfg["min_word_length"].(float64); ok {
            minLen = int(v)
        }

        if threshold < 0 || threshold > 1 {
            return nil, fmt.Errorf("keyword_presence: threshold must be between 0 and 1, got %.2f", threshold)
        }

        return NewKeywordPresenceGrader(threshold, minLen), nil
    })
}
```

Import the `graders` package in your main package to trigger `init()`:

```go
package main

import (
    _ "myapp/graders" // registers keyword_presence grader
    "github.com/greynewell/matchspec"
)
```

Now you can use the grader in YAML:

```yaml
# evals/summarization/harness.yml
graders:
  - type: keyword_presence
    name: keyword_presence
    threshold: 0.75
    config:
      threshold: 0.75
      min_word_length: 4
```

</div>

<div class="step">
<div class="step-number">Step 6</div>

## Run the eval

```bash
go run . 2>&1
# or, if registered via YAML:
matchspec run
```

Example output:

```
suite: summarization-kp
────────────────────────────────────────
keyword_presence  0.82  ✓  (≥0.70)
────────────────────────────────────────
overall           PASS
```

With `--verbose`, you'll see per-example metadata:

```
[summarization-v1] s1: keyword_presence=0.86 ✓
  matched: ["training", "compute", "40%", "structured", "pruning"]
  missing: ["reduced"]
[summarization-v1] s2: keyword_presence=0.75 ✓
  matched: ["council", "story", "downtown", "vote"]
  missing: ["approved", "buildings"]
```

</div>

## What you built

A domain-specific grader that:
- Implements the two-method `Grader` interface
- Attaches debug metadata to per-example results
- Has its own unit tests
- Is registered for use in YAML config

## Going further

- **Make it an LLM-backed grader**: Replace keyword extraction with a call to an embedding model to find semantically similar terms rather than exact keyword matches
- **Add a configuration schema validator**: Validate the `cfg` map in your `RegisterGrader` callback and return descriptive errors for missing required fields
- **Implement `GraderWithSetup`**: If your grader needs a network connection or loaded model, use `Setup`/`Teardown` to initialize and clean up — see [Custom Graders](/matchspec/docs/custom-graders/)

## Next steps

- [Custom Graders reference](/matchspec/docs/custom-graders/) — Stateful graders, HTTP clients, testing patterns
- [Graders](/matchspec/docs/graders/) — Full reference for all built-in grader types
- [CI/CD Integration](/matchspec/docs/ci-cd/) — Gate deployments on your custom grader's pass rate
