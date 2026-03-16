---
layout: tutorial-layout.njk
tool: matchspec
title: Run Your First Eval Suite
description: Write a dataset for a summarization task, wire up an exact_match grader, run matchspec, and read the results.
difficulty: beginner
time: "10 minutes"
---

# Run Your First Eval Suite

<div class="tutorial-meta">
  <span class="meta-tag">beginner</span>
  <span>10 minutes</span>
</div>

This tutorial walks through every step of a working eval suite from scratch: install matchspec, write a dataset, add a grader, run the suite, and interpret the output. You'll end with a project you can build on.

By the end you'll have:
- A `matchspec.yml` config file
- A 5-example dataset in YAML
- A harness with an `exact_match` grader
- A passing eval run

## Prerequisites

- Go 1.21 or later installed
- A terminal

You do not need an API key or external service for this tutorial — we'll use a stub model.

---

<div class="step">
<div class="step-number">Step 1</div>

## Create a project directory

```bash
mkdir my-eval-project && cd my-eval-project
go mod init my-eval-project
```

Install matchspec:

```bash
go get github.com/greynewell/matchspec
go install github.com/greynewell/matchspec/cmd/matchspec@latest
```

Verify:

```bash
matchspec --version
# matchspec v0.3.1
```
</div>

<div class="step">
<div class="step-number">Step 2</div>

## Write a dataset

Create the directory structure:

```bash
mkdir -p evals/qa
```

Create `evals/qa/dataset.yml`:

```yaml
version: 1
name: qa-basic
description: "Simple factual Q&A dataset for testing."
examples:
  - id: q1
    input: "What is the capital of France?"
    expected: "Paris"
    tags: [geography]

  - id: q2
    input: "What is the chemical symbol for water?"
    expected: "H2O"
    tags: [science]

  - id: q3
    input: "How many days are in a standard year?"
    expected: "365"
    tags: [general]

  - id: q4
    input: "What programming language was Go created at?"
    expected: "Google"
    tags: [technology]

  - id: q5
    input: "What is the square root of 144?"
    expected: "12"
    tags: [math]
```

Each example has:
- `id` — unique identifier, appears in reports when an example fails
- `input` — what you send to the model
- `expected` — the correct answer you want the model to produce
- `tags` — optional labels for filtering

</div>

<div class="step">
<div class="step-number">Step 3</div>

## Write a harness config

Create `evals/qa/harness.yml`:

```yaml
version: 1
name: qa-basic
description: "Factual QA with exact match grader."
dataset: ./dataset.yml
model:
  type: command
  command: ["go", "run", "../../cmd/stub-model/main.go"]
  input_via: stdin
graders:
  - type: exact_match
    name: exact_match
    threshold: 0.80
    config:
      trim_whitespace: true
      case_sensitive: false
```

We're using a `command` model type that calls a local Go script. Let's create that stub:

```bash
mkdir -p cmd/stub-model
```

Create `cmd/stub-model/main.go`:

```go
package main

import (
    "bufio"
    "fmt"
    "os"
    "strings"
)

// Stub model: looks up known answers, returns the input for anything else.
var answers = map[string]string{
    "what is the capital of france?":          "Paris",
    "what is the chemical symbol for water?":  "H2O",
    "how many days are in a standard year?":   "365",
    "what programming language was go created at?": "Google",
    "what is the square root of 144?":         "12",
}

func main() {
    scanner := bufio.NewScanner(os.Stdin)
    var lines []string
    for scanner.Scan() {
        lines = append(lines, scanner.Text())
    }
    input := strings.TrimSpace(strings.Join(lines, "\n"))
    key := strings.ToLower(input)

    if answer, ok := answers[key]; ok {
        fmt.Print(answer)
    } else {
        // Unknown question — return something wrong.
        fmt.Print("I don't know")
    }
}
```

This is a deterministic stub. In a real project, the `command` model would call a script that invokes an actual LLM.

<div class="callout note">
<p>You can also use <code>model: {type: http}</code> to call an OpenAI-compatible endpoint. The command model is just the easiest way to experiment without an API key.</p>
</div>

</div>

<div class="step">
<div class="step-number">Step 4</div>

## Create matchspec.yml

Create `matchspec.yml` in the project root:

```yaml
version: 1
suites:
  - name: qa
    description: "Basic factual QA eval suite."
    harnesses:
      - ./evals/qa/harness.yml
    thresholds:
      overall: 0.80
```

Your directory structure should now look like this:

```
my-eval-project/
├── go.mod
├── matchspec.yml
├── cmd/
│   └── stub-model/
│       └── main.go
└── evals/
    └── qa/
        ├── dataset.yml
        └── harness.yml
```

</div>

<div class="step">
<div class="step-number">Step 5</div>

## Run the eval suite

```bash
matchspec run
```

You should see:

```
loading suite: qa
loading harness: qa-basic (5 examples)
running model (command): go run cmd/stub-model/main.go
scoring with: exact_match

suite: qa
──────────────────────────────
exact_match  1.00  ✓  (≥0.80)
──────────────────────────────
overall      PASS

results written to: .matchspec/results/qa-20260315-143022.json
```

All 5 examples passed because our stub model returns the correct answer for every known question.

Check the exit code:

```bash
echo $?
# 0
```

Exit code 0 means all thresholds passed. This is what CI systems check.

</div>

<div class="step">
<div class="step-number">Step 6</div>

## See a failure

Let's make the suite fail to understand what that looks like. Edit `evals/qa/harness.yml` and raise the threshold higher than our score:

```yaml
graders:
  - type: exact_match
    name: exact_match
    threshold: 0.99   # changed from 0.80 to 0.99
```

Now add a hard question to `evals/qa/dataset.yml` that the stub won't know:

```yaml
  - id: q6
    input: "Who wrote the Iliad?"
    expected: "Homer"
    tags: [literature]
```

Run again:

```bash
matchspec run
```

Output:

```
suite: qa
──────────────────────────────────────────
exact_match  0.83  ✗  (≥0.99)  DELTA: -0.16
──────────────────────────────────────────
overall      FAIL

Failed graders: exact_match
  Pass rate 0.83 is below threshold 0.99 (delta: -0.16).
  Failing examples:
    q6: expected "Homer", got "I don't know"
```

```bash
echo $?
# 1
```

Exit code 1 means at least one threshold failed. Now you can see the structure of a failing report.

<div class="callout note">
<p>Reset the threshold back to 0.80 before continuing to see passing behavior.</p>
</div>

</div>

<div class="step">
<div class="step-number">Step 7</div>

## Run with verbose output

Use `--verbose` to see per-example scores during the run:

```bash
matchspec run --verbose
```

```
[qa-basic] q1: exact_match=1.00 ✓
[qa-basic] q2: exact_match=1.00 ✓
[qa-basic] q3: exact_match=1.00 ✓
[qa-basic] q4: exact_match=1.00 ✓
[qa-basic] q5: exact_match=1.00 ✓

suite: qa
──────────────────────────────
exact_match  1.00  ✓  (≥0.80)
──────────────────────────────
overall      PASS
```

Verbose mode is useful during development to spot which examples are failing before the aggregate report.

</div>

## What you built

You created a complete eval pipeline with:

- A versioned YAML dataset with 5 labeled examples
- A harness that wires the dataset to a model and a grader
- A suite config with a pass/fail threshold
- A working `matchspec run` command that exits 0 on pass, 1 on fail

## Next steps

- Replace the stub model with a real LLM using [`model.type: http`](/matchspec/docs/harnesses/#model-types)
- Add a `semantic_similarity` grader for more forgiving scoring — see [Graders](/matchspec/docs/graders/)
- Set up a GitHub Actions workflow to gate PRs on eval results — see [CI/CD Integration](/matchspec/docs/ci-cd/)
- Try the [Gate Deployments with matchspec](/matchspec/tutorials/ci-cd-gate/) tutorial
