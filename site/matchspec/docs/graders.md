---
title: Graders
description: All built-in grader types, custom grader implementation, and weighted grader composition.
---

# Graders

A grader takes a model output and an expected output and returns a score between 0.0 and 1.0. Scores are aggregated across all examples in a dataset to produce a pass rate, which is compared against a threshold to determine pass or fail.

matchspec ships with five built-in grader types. You can also implement the `Grader` interface to write your own.

## exact_match

Returns 1.0 if the model output exactly equals the expected output, 0.0 otherwise. Case-sensitive by default.

```yaml
graders:
  - type: exact_match
    name: exact_match
    threshold: 0.90
    config:
      case_sensitive: true   # default: true
      trim_whitespace: true  # default: true
```

Use `exact_match` when model outputs should be deterministic and precisely correct — for classification labels, structured codes, or short factual answers. It is a high bar, and a pass rate of 0.90 with `exact_match` is a strong signal.

## contains

Returns 1.0 if the model output contains the expected string as a substring, 0.0 otherwise.

```yaml
graders:
  - type: contains
    name: contains_answer
    threshold: 0.85
    config:
      case_sensitive: false
```

Useful when you care that the answer is present but the model is allowed to include surrounding explanation. For example, if the expected output is `"Paris"` and the model outputs `"The capital of France is Paris."`, `contains` would score this 1.0 while `exact_match` would score it 0.0.

## regex

Evaluates the model output against a regular expression. Returns 1.0 if the output matches the pattern, 0.0 otherwise.

```yaml
graders:
  - type: regex
    name: json_format
    threshold: 0.95
    config:
      pattern: '^\{.*\}$'
      flags: s    # s = dot matches newline; other flags: i (case insensitive), m (multiline)
```

Use `regex` to validate structural properties of outputs — that the model returned valid JSON, that a phone number is in the right format, that a response starts with a capital letter.

You can reference the expected field in the pattern using `{{expected}}`:

```yaml
graders:
  - type: regex
    name: contains_citation
    config:
      pattern: '(?i){{expected}}'  # case-insensitive contains using the expected value as pattern
```

## semantic_similarity

Computes the cosine similarity between the embedding of the model output and the embedding of the expected output. Returns a score between 0.0 and 1.0, where 1.0 is identical vectors.

```yaml
graders:
  - type: semantic_similarity
    name: semantic_similarity
    threshold: 0.82
    config:
      embedding_endpoint: "https://api.openai.com/v1/embeddings"
      model: "text-embedding-3-small"
      api_key_env: "OPENAI_API_KEY"  # reads from environment variable
      batch_size: 32                 # examples per embedding request (default: 32)
      timeout_seconds: 30            # per-request timeout (default: 30)
```

`semantic_similarity` is more forgiving than `exact_match` or `contains` — it captures whether the meaning is similar, not just whether the strings match. Use it for summarization, paraphrase, and open-ended generation tasks where multiple phrasings of the correct answer are acceptable.

Threshold guidance:
- **≥ 0.90**: Very similar — the model is paraphrasing closely.
- **0.80–0.89**: Similar meaning, different wording.
- **0.70–0.79**: Related but may be missing detail or adding tangential content.
- **< 0.70**: Likely wrong or off-topic.

## llm_judge

Calls a language model to evaluate the output on a rubric you define. Returns the score reported by the judge model, normalized to 0.0–1.0.

```yaml
graders:
  - type: llm_judge
    name: helpfulness
    threshold: 0.75
    config:
      endpoint: "https://api.openai.com/v1/chat/completions"
      model: "gpt-4o"
      api_key_env: "OPENAI_API_KEY"
      prompt_template: |
        You are an evaluator. Score the following response on helpfulness.

        Question: {{input}}
        Expected answer: {{expected}}
        Model response: {{output}}

        Score from 0 to 10, where 10 is perfectly helpful. Reply with only the number.
      score_parser: integer_0_10   # built-in parser; or "float_0_1", or "custom"
      timeout_seconds: 60
```

The prompt template has access to three variables:
- `{{input}}` — the original input sent to the model under test
- `{{expected}}` — the expected output from the dataset
- `{{output}}` — the actual model output being graded

Built-in score parsers:
- `integer_0_10` — parses an integer 0–10 and divides by 10
- `integer_0_5` — parses an integer 0–5 and divides by 5
- `float_0_1` — parses a float between 0 and 1

`llm_judge` is powerful but expensive and slow. Use it selectively on the subset of examples where cheaper graders are insufficient.

## Weighted grader composition

Run multiple graders on the same output and combine their scores with weights:

```yaml
graders:
  - type: semantic_similarity
    name: semantic_similarity
    weight: 0.7
    threshold: 0.80
  - type: exact_match
    name: exact_match
    weight: 0.3
    threshold: 0.60
```

When weights are specified, the overall score for each example is a weighted average of the individual grader scores. The per-grader thresholds still apply — an example can pass the overall threshold while failing one grader's individual threshold.

If no weights are specified, all graders are treated as independent pass/fail checks and each must independently meet its threshold.

## Custom graders

Implement the `Grader` interface to write a grader in Go:

```go
// Grader scores a single model output.
type Grader interface {
    Name() string
    Score(ctx context.Context, input, expected, output string) (Score, error)
}

// Score is the result of grading one example.
type Score struct {
    Value    float64           // 0.0–1.0
    Passed   bool              // true if Value >= threshold
    Metadata map[string]any    // optional: attach reasoning, debug info, etc.
}
```

A complete custom grader example:

```go
package graders

import (
    "context"
    "strings"

    "github.com/greynewell/matchspec"
)

// WordOverlapGrader scores by the fraction of expected words present in the output.
type WordOverlapGrader struct {
    threshold float64
}

func NewWordOverlapGrader(threshold float64) *WordOverlapGrader {
    return &WordOverlapGrader{threshold: threshold}
}

func (g *WordOverlapGrader) Name() string { return "word_overlap" }

func (g *WordOverlapGrader) Score(ctx context.Context, input, expected, output string) (matchspec.Score, error) {
    expectedWords := tokenize(expected)
    outputWords := tokenize(output)

    if len(expectedWords) == 0 {
        return matchspec.Score{Value: 1.0, Passed: true}, nil
    }

    outputSet := make(map[string]bool, len(outputWords))
    for _, w := range outputWords {
        outputSet[w] = true
    }

    matches := 0
    for _, w := range expectedWords {
        if outputSet[w] {
            matches++
        }
    }

    score := float64(matches) / float64(len(expectedWords))
    return matchspec.Score{
        Value:  score,
        Passed: score >= g.threshold,
        Metadata: map[string]any{
            "expected_words": len(expectedWords),
            "matched_words":  matches,
        },
    }, nil
}

func tokenize(s string) []string {
    // Simple lowercase word tokenizer.
    s = strings.ToLower(s)
    return strings.Fields(s)
}
```

Register your custom grader for use in YAML harness configs:

```go
func init() {
    matchspec.RegisterGrader("word_overlap", func(config map[string]any) (matchspec.Grader, error) {
        threshold, _ := config["threshold"].(float64)
        if threshold == 0 {
            threshold = 0.70
        }
        return NewWordOverlapGrader(threshold), nil
    })
}
```

After registration, you can reference the grader type in YAML:

```yaml
graders:
  - type: word_overlap
    name: word_overlap
    threshold: 0.75
    config:
      threshold: 0.75
```

For more on custom graders, including stateful graders and testing, see [Custom Graders](/matchspec/docs/custom-graders/).

## Grader configuration reference

All grader configurations support these common fields:

| Field | Type | Required | Description |
|---|---|---|---|
| `type` | string | yes | Grader type identifier. |
| `name` | string | yes | Display name used in reports. Must be unique within a harness. |
| `threshold` | float | no | Per-grader pass threshold (0.0–1.0). Overrides the suite-level threshold for this grader. |
| `weight` | float | no | Weight for weighted composition. If any grader has a weight, all must have weights. |
| `config` | object | no | Grader-specific configuration. See per-grader sections above. |
