---
title: "Compiling an Eval Dataset"
description: "Annotate markdown files with evaluation frontmatter, run them through the schemaflux pipeline, and produce a structured JSON dataset ready for matchspec."
difficulty: intermediate
time: 20 min
tags:
  - JSON backend
  - matchspec
  - eval
---

# Compiling an Eval Dataset

<div class="tutorial-meta">
  <span class="meta-tag">intermediate</span>
  <span class="tutorial-time">20 min</span>
</div>

In this tutorial you will create a set of eval examples as annotated markdown files, compile them into a structured JSON dataset with schemaflux, validate that every example has the required fields, and load the dataset into matchspec.

**What you will build:**

- 6 eval examples across two categories (summarization and classification)
- A schemaflux config that validates required eval fields
- A compiled `dataset.json` with all examples and computed similarity metadata
- A Go snippet that loads the dataset into matchspec

**Prerequisites:** schemaflux and Go installed. Familiarity with YAML frontmatter.

---

<div class="step">
<div class="step-number">Step 1</div>

## Create the project

```bash
mkdir eval-dataset
cd eval-dataset
mkdir -p examples/summarization examples/classification
```

</div>

<div class="step">
<div class="step-number">Step 2</div>

## Write the eval examples

Each example is a markdown file. The structured fields go in frontmatter; the markdown body provides context for human reviewers.

**examples/summarization/short-paragraph.md**

```markdown
---
title: "Summarization — short paragraph"
difficulty: easy
category: summarization
tags:
  - summarization
  - factual
input: |
  The water cycle describes the continuous movement of water within
  the Earth and atmosphere. It involves evaporation, condensation,
  precipitation, and collection.
expected_output: "Water continuously moves through the Earth and atmosphere via evaporation, condensation, precipitation, and collection."
grader: semantic_similarity
min_score: 0.80
---

Baseline easy example. Single short paragraph, one-sentence summary.
Straightforward factual content with no ambiguity.
```

**examples/summarization/technical-document.md**

```markdown
---
title: "Summarization — technical document excerpt"
difficulty: medium
category: summarization
tags:
  - summarization
  - technical
input: |
  The Transmission Control Protocol (TCP) provides reliable, ordered,
  and error-checked delivery of a stream of bytes between applications
  running on hosts communicating via an IP network. TCP is
  connection-oriented, and a connection between client and server is
  established before data can be sent. The server must be listening
  (passive open) for connection requests from clients before a
  connection is established.
expected_output: "TCP provides reliable, ordered byte-stream delivery over IP, using connection-oriented sessions established before data transfer begins."
grader: semantic_similarity
min_score: 0.75
---

Technical content with domain-specific terminology. The model must retain
key technical facts (reliable, ordered, connection-oriented) without
hallucinating additional details.
```

**examples/summarization/multi-paragraph.md**

```markdown
---
title: "Summarization — multi-paragraph article"
difficulty: hard
category: summarization
tags:
  - summarization
  - long-form
input: |
  Climate change refers to long-term shifts in temperatures and weather
  patterns. These shifts may be natural, such as through variations in
  the solar cycle. But since the 1800s, human activities have been the
  main driver of climate change, primarily due to burning fossil fuels
  like coal, oil, and gas.

  Burning fossil fuels generates greenhouse gas emissions that act like
  a blanket wrapped around the Earth, trapping the sun's heat and
  raising temperatures.

  The main greenhouse gases that are causing climate change include
  carbon dioxide and methane. These come from using gasoline for
  driving a car or coal for heating a building. Clearing land and
  forests can also release carbon dioxide. Landfills for garbage are
  a major source of methane emissions. Energy, industry, transport,
  buildings, agriculture, and land use are among the main emitters.
expected_output: "Climate change is caused primarily by human fossil fuel use since the 1800s, which releases greenhouse gases that trap heat and raise global temperatures."
grader: semantic_similarity
min_score: 0.70
---

Multi-paragraph source with redundant information. The summary should
synthesize across paragraphs and capture the causal chain without
listing every detail.
```

**examples/classification/positive-review.md**

```markdown
---
title: "Sentiment classification — positive review"
difficulty: easy
category: classification
tags:
  - classification
  - sentiment
input: "This product exceeded my expectations. The build quality is excellent and it arrived ahead of schedule. Highly recommend."
expected_output: "positive"
grader: exact_match
min_score: 1.0
---

Unambiguous positive sentiment. Clear positive indicators: "exceeded expectations",
"excellent", "highly recommend". Expected output is exact match.
```

**examples/classification/negative-review.md**

```markdown
---
title: "Sentiment classification — negative review"
difficulty: easy
category: classification
tags:
  - classification
  - sentiment
input: "Terrible experience. The item broke after two uses and customer service never responded to my refund request."
expected_output: "negative"
grader: exact_match
min_score: 1.0
---

Unambiguous negative sentiment. Clear negative indicators: "terrible",
"broke", "never responded".
```

**examples/classification/mixed-review.md**

```markdown
---
title: "Sentiment classification — mixed review"
difficulty: hard
category: classification
tags:
  - classification
  - sentiment
  - ambiguous
input: "The hardware is well-made and looks great, but the software is buggy and the setup process took three hours."
expected_output: "mixed"
grader: exact_match
min_score: 1.0
---

Genuinely mixed sentiment — positive hardware assessment, negative software
and setup experience. Tests whether the model can identify mixed vs defaulting
to positive or negative.
```

</div>

<div class="step">
<div class="step-number">Step 3</div>

## Write the config

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
      layout: ""
    categories:
      enabled: true
      urlPrefix: /categories/
      layout: ""

  relationships:
    topN: 3
    minScore: 0.10
    weights:
      tagOverlap: 0.6
      sectionMatch: 0.3
      titleTokenOverlap: 0.1

  schemaGen:
    enabled: true
    requiredThreshold: 0.9

  validate:
    schema: true
    rules:
      - name: "require-eval-fields"
        require:
          - title
          - difficulty
          - input
          - expected_output
          - grader
          - min_score

backends:
  html:
    enabled: false

  json:
    enabled: true
    combined: true
    combinedFile: dataset.json
    fields:
      - id
      - title
      - section
      - tags
      - url
      - related
      - frontmatter

  sitemap:
    enabled: false
```

</div>

<div class="step">
<div class="step-number">Step 4</div>

## Validate the examples

Before building the full dataset, run validate to check that every example has the required fields:

```bash
schemaflux validate
```

Expected output:

```
validating 6 entities...
0 violations
0 warnings
ok
```

If a field is missing, you'll see a diagnostic like:

```
error: entity "summarization/short-paragraph" is missing required field "min_score"
       rule: require-eval-fields
```

</div>

<div class="step">
<div class="step-number">Step 5</div>

## Compile the dataset

```bash
schemaflux build
```

```
parsing   6 files
pass 1/12 slugify
pass 2/12 sort
pass 3/12 enrich
pass 4/12 taxonomy
pass 5/12 relationships
pass 6/12 graph
pass 7/12 url-resolve
pass 8/12 schema-gen
pass 9/12 validate        6 entities  →  0 violations
pass 11/12 emit-json      1 file

output    1 file in ./_dataset/
built in  11ms
```

</div>

<div class="step">
<div class="step-number">Step 6</div>

## Inspect the output

Open `_dataset/dataset.json`. Each entity looks like:

```json
{
  "id": "summarization/short-paragraph",
  "title": "Summarization — short paragraph",
  "section": "summarization",
  "tags": [
    { "name": "summarization", "slug": "summarization" },
    { "name": "factual", "slug": "factual" }
  ],
  "url": "/summarization/short-paragraph/",
  "related": [
    {
      "id": "summarization/technical-document",
      "url": "/summarization/technical-document/",
      "title": "Summarization — technical document excerpt"
    }
  ],
  "frontmatter": {
    "title": "Summarization — short paragraph",
    "difficulty": "easy",
    "category": "summarization",
    "input": "The water cycle...",
    "expected_output": "Water continuously moves...",
    "grader": "semantic_similarity",
    "min_score": 0.8
  }
}
```

Notice the `related` field: schemaflux computed that `technical-document` is similar to `short-paragraph` based on shared tags and section. This metadata is useful for auditing your dataset for near-duplicates.

</div>

<div class="step">
<div class="step-number">Step 7</div>

## Load into matchspec

Create a Go program that loads the compiled dataset and runs it through matchspec:

**main.go**

```go
package main

import (
    "encoding/json"
    "fmt"
    "log"
    "os"

    "github.com/greynewell/matchspec"
)

// SchemafluxEntity mirrors the JSON output structure
type SchemafluxEntity struct {
    ID          string         `json:"id"`
    Title       string         `json:"title"`
    Section     string         `json:"section"`
    Frontmatter map[string]any `json:"frontmatter"`
}

type SchemafluxDataset struct {
    Count    int                 `json:"count"`
    Entities []SchemafluxEntity  `json:"entities"`
}

func loadExamples(path string) ([]matchspec.Example, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, fmt.Errorf("open dataset: %w", err)
    }
    defer f.Close()

    var dataset SchemafluxDataset
    if err := json.NewDecoder(f).Decode(&dataset); err != nil {
        return nil, fmt.Errorf("decode dataset: %w", err)
    }

    examples := make([]matchspec.Example, 0, len(dataset.Entities))
    for _, e := range dataset.Entities {
        fm := e.Frontmatter
        example := matchspec.Example{
            ID:             e.ID,
            Input:          stringOr(fm, "input", ""),
            ExpectedOutput: stringOr(fm, "expected_output", ""),
            Metadata: map[string]any{
                "title":      e.Title,
                "difficulty": fm["difficulty"],
                "grader":     fm["grader"],
                "min_score":  fm["min_score"],
                "section":    e.Section,
            },
        }
        examples = append(examples, example)
    }
    return examples, nil
}

func stringOr(m map[string]any, key, fallback string) string {
    if v, ok := m[key]; ok {
        if s, ok := v.(string); ok {
            return s
        }
    }
    return fallback
}

func main() {
    examples, err := loadExamples("./_dataset/dataset.json")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Loaded %d examples\n", len(examples))

    // Group by section for targeted eval runs
    bySec := make(map[string][]matchspec.Example)
    for _, ex := range examples {
        sec := ex.Metadata["section"].(string)
        bySec[sec] = append(bySec[sec], ex)
    }
    for sec, exs := range bySec {
        fmt.Printf("  %s: %d examples\n", sec, len(exs))
    }
}
```

Run it:

```bash
go run main.go
# Loaded 6 examples
#   summarization: 3 examples
#   classification: 3 examples
```

</div>

---

## What you built

- 6 annotated eval examples in two categories
- A schemaflux pipeline that validates required fields and computes similarity metadata
- A compiled `dataset.json` with computed `related` links between similar examples
- A Go loader that maps the JSON output to matchspec's Example type

## Tips for larger datasets

- Keep examples in section directories by task type (`summarization/`, `code-review/`, `classification/`) — schemaflux uses these sections for schema inference and prev/next ordering
- Use `schemaflux graph --filter summarization` to inspect section contents before a full build
- Set `relationships.minScore: 0.05` to surface near-duplicates, then remove or deduplicate them
- Use `validate --format json` in CI to catch missing fields early

## Next steps

- [Compiling Eval Datasets guide](/schemaflux/docs/eval-datasets/) — deeper reference for eval dataset configuration
- [matchspec](/matchspec/) — run graders and thresholds against your compiled dataset
- [Writing a Custom Backend tutorial](/schemaflux/tutorials/custom-backend/) — emit a JSONL file per category instead of a combined JSON
