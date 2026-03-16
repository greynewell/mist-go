---
title: Datasets
description: Define evaluation inputs and expected outputs in YAML or Go — loading, fields, versioning, and seeding.
---

# Datasets

A dataset is a collection of examples. Each example has an input (what you send to the model), an expected output (what you want back), and optional metadata. Datasets are the foundation of every eval run — they determine what gets tested and what "correct" means.

## YAML format

The most common way to define a dataset is in a YAML file:

```yaml
version: 1
name: summarization-v2
description: "Summarization eval covering science, policy, and health domains."
examples:
  - id: ex-001
    input: |
      Summarize in one sentence:
      Researchers developed a method that reduces neural network
      training compute by 40% using structured pruning.
    expected: "Researchers reduced neural network training compute by 40% with structured pruning."
    metadata:
      source: "arxiv:2024.12345"
      difficulty: easy
    tags:
      - science
      - ml

  - id: ex-002
    input: |
      Summarize in one sentence:
      The city council voted 7-2 to approve 8-story residential
      buildings downtown, reversing a 1987 height cap of 4 stories.
    expected: "The city council approved taller downtown buildings, reversing a decades-old restriction."
    tags:
      - policy

  - id: ex-003
    input: |
      Summarize in one sentence:
      A trial of 10,000 patients found the new drug cut readmissions
      by 23% with no significant adverse events.
    expected: "A large trial found the new drug reduced hospital readmissions by 23% safely."
    tags:
      - health
```

### Top-level fields

| Field | Type | Required | Description |
|---|---|---|---|
| `version` | integer | yes | Schema version. Must be `1`. |
| `name` | string | yes | Identifier for this dataset. Used in reports. |
| `description` | string | no | Human-readable description. |
| `examples` | array | yes | List of examples. |

### Example fields

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Unique identifier within the dataset. Used in per-example results. |
| `input` | string | yes | The input to send to the model. Can be a prompt, a JSON payload, or any string. |
| `expected` | string | yes | The expected model output. Graders compare the actual output against this value. |
| `metadata` | object | no | Arbitrary key-value pairs attached to the example. Available to graders and in results. |
| `tags` | array of strings | no | Labels for filtering. Run evals only on examples with specific tags using `--tags`. |

## Go struct format

You can also define datasets entirely in Go:

```go
package evals

import "github.com/greynewell/matchspec"

var SummarizationDataset = matchspec.Dataset{
    Name:        "summarization-v2",
    Description: "Summarization eval covering science, policy, and health.",
    Examples: []matchspec.Example{
        {
            ID:       "ex-001",
            Input:    "Summarize in one sentence: Researchers reduced neural network training compute by 40% using structured pruning.",
            Expected: "Researchers cut neural network training compute by 40% with structured pruning.",
            Tags:     []string{"science", "ml"},
            Metadata: map[string]any{
                "source":     "arxiv:2024.12345",
                "difficulty": "easy",
            },
        },
        {
            ID:       "ex-002",
            Input:    "Summarize in one sentence: The city council approved 8-story buildings downtown, reversing a 1987 cap.",
            Expected: "The city council approved taller buildings downtown, overturning a decades-old height limit.",
            Tags:     []string{"policy"},
        },
    },
}
```

Go-defined datasets can be used directly in harnesses without any file I/O:

```go
harness := matchspec.Harness{
    Name:    "summarization-v2",
    Dataset: evals.SummarizationDataset,
    // ...
}
```

This is useful when you want to co-locate datasets with the Go code that uses them, or when you want dataset examples to reference non-string inputs that YAML cannot express cleanly.

## Loading datasets from files

When configuring harnesses via YAML, reference datasets by path:

```yaml
# harness.yml
dataset: ./dataset.yml
```

Relative paths are resolved from the directory containing the harness file. You can also use absolute paths or paths relative to the root of the project (where `matchspec.yml` lives).

To load a dataset from Go code:

```go
ds, err := matchspec.LoadDatasetFile("./evals/summarization/dataset.yml")
if err != nil {
    log.Fatal(err)
}
```

## Dataset versioning

Datasets should be versioned alongside your prompts and model configs. Best practices:

- **Use semantic names with version suffixes**: `summarization-v1`, `summarization-v2`. When you add or modify examples in ways that change the comparison baseline, increment the version.
- **Keep old datasets**: Don't delete previous versions of a dataset when you create a new one. Keeping both lets you run historical comparisons.
- **Track in source control**: Dataset YAML files should live in the same repository as your application code. Changes to datasets should go through code review.
- **Embed in your binary when possible**: Use `go:embed` to include dataset files in your binary so that CI workers don't need to fetch them:

```go
import _ "embed"

//go:embed evals/summarization/dataset.yml
var datasetYAML []byte

func loadDataset() (matchspec.Dataset, error) {
    return matchspec.ParseDatasetYAML(datasetYAML)
}
```

## Filtering by tags

Run a suite against a subset of examples using `--tags`:

```bash
# Only run examples tagged "science"
matchspec run --tags science

# Run examples tagged "science" OR "ml"
matchspec run --tags science,ml
```

Tag filtering applies across all harnesses in the suite. Examples without matching tags are skipped, and the pass rate is computed only over the matching examples.

## Seeding datasets from existing data

If you have logs of real model inputs and human-labeled outputs, you can seed a dataset from them. matchspec provides a `matchspec.DatasetBuilder` for this pattern:

```go
builder := matchspec.NewDatasetBuilder("production-sample-v1")

for _, logEntry := range productionLogs {
    if logEntry.HumanLabel != "" {
        builder.Add(matchspec.Example{
            ID:       logEntry.RequestID,
            Input:    logEntry.Prompt,
            Expected: logEntry.HumanLabel,
            Metadata: map[string]any{
                "timestamp": logEntry.Timestamp,
                "user_segment": logEntry.UserSegment,
            },
        })
    }
}

dataset := builder.Build()

// Optionally write to YAML for review and version control.
if err := matchspec.WriteDatasetFile(dataset, "./evals/production-sample-v1.yml"); err != nil {
    log.Fatal(err)
}
```

Seeding from production logs is a powerful way to build representative datasets, but review the examples before committing them — production data may contain sensitive information or adversarial inputs that should not live in source control unmodified.

## Large datasets

For datasets with thousands of examples, YAML files become unwieldy. matchspec supports JSON Lines (`.jsonl`) format, where each line is a JSON object representing one example:

```jsonl
{"id":"ex-001","input":"Summarize: ...","expected":"...","tags":["science"]}
{"id":"ex-002","input":"Summarize: ...","expected":"...","tags":["policy"]}
```

Load a JSONL dataset the same way:

```go
ds, err := matchspec.LoadDatasetFile("./evals/large-dataset.jsonl")
```

matchspec detects the format from the file extension (`.yml`/`.yaml` for YAML, `.jsonl` for JSON Lines).
