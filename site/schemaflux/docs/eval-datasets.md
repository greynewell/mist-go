---
title: "Compiling Eval Datasets"
description: "Compile annotated markdown files into structured JSON datasets for matchspec. Frontmatter schema, the compilation pipeline, and output format."
---

# Compiling Eval Datasets

schemaflux can compile a directory of annotated markdown files into a structured JSON dataset ready for use with [matchspec](/matchspec/). This workflow is useful when you want to author eval examples as human-readable markdown but consume them as machine-readable JSON in your eval pipeline.

## Why markdown for evals

Markdown is a good authoring format for eval examples because:

- Examples with long, multi-paragraph inputs are readable in their natural form
- YAML frontmatter cleanly separates structured metadata (grader config, expected output, difficulty) from free-text content (the input or prompt)
- Git-based review workflows work well on markdown files
- Authors can add context, notes, and explanations in the markdown body without polluting the structured fields

The schemaflux pipeline adds value beyond simple YAML-to-JSON conversion: the relationship-scoring and taxonomy passes compute similarity between examples, group them by tag, and validate that every required field is present before the dataset is emitted.

## Frontmatter schema for eval examples

Define a consistent frontmatter schema for your eval examples. A minimal schema:

```markdown
---
title: "Summarization — short article"
difficulty: easy
category: summarization
tags:
  - summarization
  - factual
input: |
  The water cycle describes the continuous movement of water within the Earth
  and atmosphere. It involves processes such as evaporation, condensation,
  precipitation, and collection.
expected_output: "Water continuously moves through the Earth and atmosphere via evaporation, condensation, precipitation, and collection."
grader: semantic_similarity
min_score: 0.80
---

A straightforward summarization example. The input is a short factual paragraph.
The expected output is a one-sentence summary.
```

For examples with very long inputs, put the metadata in frontmatter and the input in the markdown body, then use the `content` field from the JSON output as the input:

```markdown
---
title: "Code review — Go concurrency bug"
difficulty: hard
category: code-review
tags:
  - code-review
  - go
  - concurrency
expected_output: "The code has a data race on the counter variable. Use sync/atomic or a mutex."
grader: llm_judge
rubric: "Identifies the data race, names the affected variable, and suggests a correct fix."
min_score: 0.75
---

Review the following Go code and identify any concurrency issues:

```go
var counter int

func increment() {
    counter++
}

func main() {
    for i := 0; i < 1000; i++ {
        go increment()
    }
}
```

## Project structure

```
eval-dataset/
  examples/
    summarization/
      short-factual.md
      long-article.md
      technical-doc.md
    code-review/
      concurrency-bug.md
      nil-pointer.md
    classification/
      sentiment-positive.md
      sentiment-negative.md
  schemaflux.yml
```

## Configuration

Configure schemaflux to use the JSON backend and validate required eval fields:

```yaml
# schemaflux.yml
site:
  title: "My Eval Dataset"

input:
  dir: ./examples

output:
  dir: ./_dataset

passes:
  taxonomy:
    tags:
      enabled: true
      urlPrefix: /tags/
      layout: ""           # no HTML output needed
    categories:
      enabled: true
      urlPrefix: /categories/
      layout: ""

  relationships:
    topN: 5
    minScore: 0.15

  schemaGen:
    enabled: true
    requiredThreshold: 0.9   # fields present on 90%+ of examples are required

  validate:
    schema: true
    rules:
      - name: "require-eval-fields"
        require:
          - title
          - difficulty
          - expected_output
          - grader

backends:
  html:
    enabled: false           # no HTML output for datasets

  json:
    enabled: true
    combined: true
    combinedFile: dataset.json
    fields:
      - id
      - title
      - description
      - content
      - date
      - tags
      - section
      - frontmatter
      - related
      - url

  sitemap:
    enabled: false
```

## Running the compilation

```bash
schemaflux build

# Output:
# parsing   24 files
# pass 1/12 slugify
# ...
# pass 9/12 validate        24 entities  →  0 violations
# pass 11/12 emit-json      1 file
#
# output    1 file in ./_dataset/
# built in  18ms
```

The output file `_dataset/dataset.json` is a structured JSON file containing all examples.

## Output format

The combined `dataset.json`:

```json
{
  "generated_at": "2026-03-15T10:00:00Z",
  "count": 24,
  "entities": [
    {
      "id": "summarization/short-factual",
      "title": "Summarization — short article",
      "description": null,
      "content": "<p>A straightforward summarization example...</p>",
      "date": null,
      "tags": [
        { "name": "summarization", "slug": "summarization" },
        { "name": "factual", "slug": "factual" }
      ],
      "section": "summarization",
      "url": "/summarization/short-factual/",
      "related": [
        {
          "id": "summarization/long-article",
          "url": "/summarization/long-article/",
          "title": "Summarization — long article"
        }
      ],
      "frontmatter": {
        "title": "Summarization — short article",
        "difficulty": "easy",
        "category": "summarization",
        "input": "The water cycle...",
        "expected_output": "Water continuously moves...",
        "grader": "semantic_similarity",
        "min_score": 0.80
      }
    }
  ]
}
```

## Loading into matchspec

The compiled dataset can be loaded directly into matchspec as a custom dataset. Map the schemaflux output fields to matchspec's `Example` type:

```go
package main

import (
    "encoding/json"
    "os"

    "github.com/greynewell/matchspec"
)

type SchemafluxEntity struct {
    ID          string         `json:"id"`
    Title       string         `json:"title"`
    Frontmatter map[string]any `json:"frontmatter"`
}

type SchemafluxDataset struct {
    Count    int                 `json:"count"`
    Entities []SchemafluxEntity  `json:"entities"`
}

func LoadDataset(path string) ([]matchspec.Example, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()

    var sf SchemafluxDataset
    if err := json.NewDecoder(f).Decode(&sf); err != nil {
        return nil, err
    }

    examples := make([]matchspec.Example, 0, len(sf.Entities))
    for _, e := range sf.Entities {
        fm := e.Frontmatter
        ex := matchspec.Example{
            ID:             e.ID,
            Input:          stringField(fm, "input"),
            ExpectedOutput: stringField(fm, "expected_output"),
            Metadata: map[string]any{
                "title":      e.Title,
                "difficulty": fm["difficulty"],
                "grader":     fm["grader"],
                "min_score":  fm["min_score"],
            },
        }
        examples = append(examples, ex)
    }
    return examples, nil
}

func stringField(m map[string]any, key string) string {
    if v, ok := m[key]; ok {
        if s, ok := v.(string); ok {
            return s
        }
    }
    return ""
}
```

## Filtering by difficulty or tag

Use `schemaflux graph` to inspect the compiled IR before building, or filter in the loader:

```bash
# See all examples in the summarization section
schemaflux graph --filter summarization --fields id,title,frontmatter

# See difficulty distribution
schemaflux graph --no-pretty | jq '.entities | group_by(.frontmatter.difficulty) | map({difficulty: .[0].frontmatter.difficulty, count: length})'
```

## Validating the dataset

Run `schemaflux validate` in CI to ensure every example has the required fields before the dataset is used:

```bash
schemaflux validate --format json
```

With the validation rules configured above, this will fail if any example is missing `title`, `difficulty`, `expected_output`, or `grader`. The JSON output lists every violation with the entity ID and the missing field name, making it easy to surface in CI logs.

## Incremental authoring workflow

When authoring a large dataset:

1. Write examples in markdown, committing as you go
2. Run `schemaflux validate` before each push to catch missing fields
3. Run `schemaflux build` to produce the final dataset JSON
4. Load the dataset into matchspec for eval runs
5. Use the `related` field in the output to find similar examples — useful for auditing for duplicate inputs or near-duplicates that could inflate pass rates
