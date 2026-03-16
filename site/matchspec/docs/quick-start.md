---
title: Quick Start
description: Run your first eval suite in five minutes — install matchspec, write a dataset and grader, and see results.
---

# Quick Start

This guide walks through the complete flow: install matchspec, write a dataset, write a grader, create a harness, and run `matchspec run`. By the end you'll have a working eval suite that exits 0 on pass and non-zero on fail.

## 1. Install

Install the Go package and the CLI:

```bash
go get github.com/greynewell/matchspec
go install github.com/greynewell/matchspec/cmd/matchspec@latest
```

Verify the CLI is available:

```bash
matchspec --version
# matchspec v0.1.0
```

## 2. Initialize a project

Run `matchspec init` in your project directory to create a starter config file:

```bash
matchspec init
# created matchspec.yml
```

This creates a minimal `matchspec.yml`:

```yaml
version: 1
suites:
  - name: default
    harnesses: []
    thresholds:
      overall: 0.80
```

## 3. Write a dataset

Create a YAML dataset file at `evals/summarization/dataset.yml`:

```yaml
version: 1
name: summarization-basic
examples:
  - id: ex-001
    input: |
      Summarize this article in one sentence:
      Researchers at MIT have developed a new technique for training
      neural networks that reduces compute requirements by 40% while
      maintaining accuracy within 2% of baseline. The method uses
      structured pruning during the warm-up phase.
    expected: "MIT researchers reduced neural network training compute by 40% with minimal accuracy loss using structured pruning."
    tags:
      - science
      - ml

  - id: ex-002
    input: |
      Summarize this article in one sentence:
      The city council voted 7-2 to approve a new zoning ordinance
      allowing residential buildings up to 8 stories in the downtown
      corridor, reversing a 1987 policy that had capped height at 4 stories.
    expected: "The city council approved taller downtown residential buildings, reversing a decades-old height restriction."
    tags:
      - local-government

  - id: ex-003
    input: |
      Summarize this article in one sentence:
      A study of 10,000 patients found that the new drug reduced
      hospital readmission rates by 23% compared to the standard
      treatment, with no significant increase in adverse events.
    expected: "A large study found the new drug cut hospital readmissions by 23% without additional safety concerns."
    tags:
      - health
```

## 4. Write a harness config

Create `evals/summarization/harness.yml`:

```yaml
version: 1
name: summarization-v1
dataset: ./dataset.yml
model:
  type: http
  endpoint: "http://localhost:8080/v1/completions"
  headers:
    Authorization: "Bearer ${MODEL_API_KEY}"
graders:
  - type: semantic_similarity
    name: semantic_similarity
    threshold: 0.80
    config:
      embedding_endpoint: "http://localhost:8080/v1/embeddings"
      model: "text-embedding-3-small"
concurrency: 4
```

If you want to run without an actual model during development, you can use the Go API to stub the model (see step 5b below).

## 5a. Run from the CLI

Update `matchspec.yml` to reference your harness:

```yaml
version: 1
suites:
  - name: summarization
    harnesses:
      - ./evals/summarization/harness.yml
    thresholds:
      overall: 0.80
```

Then run:

```bash
matchspec run
```

Output:

```
loading suite: summarization
loading harness: summarization-v1 (3 examples)
running model: http://localhost:8080/v1/completions
scoring with: semantic_similarity

suite: summarization
─────────────────────────────────
semantic_similarity  0.86  ✓  (≥0.80)
─────────────────────────────────
overall              PASS

results written to: .matchspec/results/summarization-20260315-143022.json
```

## 5b. Run from Go code

You can also drive matchspec entirely from Go — useful in tests or when you want to stub the model:

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/greynewell/matchspec"
)

func main() {
    // Define a dataset inline.
    dataset := matchspec.Dataset{
        Name: "summarization-basic",
        Examples: []matchspec.Example{
            {
                ID:       "ex-001",
                Input:    "Summarize in one sentence: Go is a statically typed, compiled language designed at Google.",
                Expected: "Go is a statically typed compiled language created at Google.",
            },
            {
                ID:       "ex-002",
                Input:    "Summarize in one sentence: The Eiffel Tower is a wrought-iron lattice tower on the Champ de Mars in Paris, completed in 1889.",
                Expected: "The Eiffel Tower is a 19th-century iron tower in Paris.",
            },
        },
    }

    // Stub model for testing.
    model := matchspec.ModelFunc(func(ctx context.Context, input string) (string, error) {
        // In production this would call your actual model.
        return "Go is a compiled language made by Google.", nil
    })

    // Use semantic similarity grader.
    grader := matchspec.NewSemanticSimilarityGrader(matchspec.SemanticSimilarityConfig{
        EmbeddingEndpoint: "http://localhost:8080/v1/embeddings",
        Model:             "text-embedding-3-small",
        Threshold:         0.80,
    })

    harness := matchspec.Harness{
        Name:    "summarization-v1",
        Dataset: dataset,
        Model:   model,
        Graders: []matchspec.Grader{grader},
    }

    suite := matchspec.Suite{
        Name:     "summarization",
        Harnesses: []matchspec.Harness{harness},
        Thresholds: matchspec.Thresholds{
            Overall: 0.80,
        },
    }

    result, err := suite.Run(context.Background())
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Overall: %s\n", result.Verdict)
    for _, gr := range result.GraderResults {
        fmt.Printf("  %s: %.2f (threshold %.2f)\n", gr.Name, gr.Score, gr.Threshold)
    }

    if !result.Passed() {
        // Exit non-zero — suitable for use in tests or main().
        log.Fatal("suite failed")
    }
}
```

## 6. Interpret the results

The report shows one row per grader with:

- **Score** — aggregate pass rate across all examples (0.0–1.0)
- **Status** — `✓` if the score meets the threshold, `✗` if it does not
- **Threshold** — the minimum score required to pass

If any grader is below its threshold, the overall verdict is `FAIL` and the exit code is non-zero.

Results are also written to `.matchspec/results/` as JSON for use in CI reporting, badge generation, and historical tracking. The JSON format is documented in the [HTTP API](/matchspec/docs/http-api/) reference.

## Next steps

- [Datasets](/matchspec/docs/datasets/) — All dataset fields, YAML vs Go, versioning, and loading options.
- [Graders](/matchspec/docs/graders/) — All built-in graders and how to compose them.
- [CI/CD Integration](/matchspec/docs/ci-cd/) — Run evals on every PR and fail the build on regression.
