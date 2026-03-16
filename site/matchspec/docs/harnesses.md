---
title: Harnesses
description: Wire datasets and graders into a runnable eval suite — harness config, the Model interface, concurrency, and retry.
---

# Harnesses

A harness is the wiring layer of matchspec. It binds a dataset, a model, and one or more graders together into a runnable eval. When you run a harness, matchspec calls your model on each dataset example, scores each output with the configured graders, and returns per-example results that roll up into aggregate pass rates.

## Harness config (YAML)

Define a harness in a YAML file:

```yaml
version: 1
name: summarization-v2
description: "Summarization quality eval using semantic similarity."

# Path to a dataset file, or inline dataset.
dataset: ./dataset.yml

# Model configuration.
model:
  type: http
  endpoint: "https://api.openai.com/v1/chat/completions"
  api_key_env: "OPENAI_API_KEY"
  request_template: |
    {
      "model": "gpt-4o-mini",
      "messages": [
        {"role": "user", "content": "{{input}}"}
      ],
      "max_tokens": 150
    }
  response_path: "choices[0].message.content"

# Graders to run on each output.
graders:
  - type: semantic_similarity
    name: semantic_similarity
    threshold: 0.82
    config:
      embedding_endpoint: "https://api.openai.com/v1/embeddings"
      model: "text-embedding-3-small"
      api_key_env: "OPENAI_API_KEY"

  - type: exact_match
    name: exact_match
    threshold: 0.60

# Execution settings.
concurrency: 8       # parallel model calls (default: 4)
timeout_seconds: 30  # per-example timeout (default: 30)
retries: 2           # retry failed model calls (default: 0)
retry_delay_ms: 500  # delay between retries (default: 250)
```

### Harness fields

| Field | Type | Required | Description |
|---|---|---|---|
| `version` | integer | yes | Schema version. Must be `1`. |
| `name` | string | yes | Identifier for this harness. Used in reports. |
| `description` | string | no | Human-readable description. |
| `dataset` | string or object | yes | Path to a dataset file, or an inline dataset definition. |
| `model` | object | yes | Model configuration. See [Model types](#model-types) below. |
| `graders` | array | yes | List of grader configurations. See [Graders](/matchspec/docs/graders/). |
| `concurrency` | integer | no | Number of parallel model calls. Default: `4`. |
| `timeout_seconds` | integer | no | Per-example timeout in seconds. Default: `30`. |
| `retries` | integer | no | Number of retries for failed model calls. Default: `0`. |
| `retry_delay_ms` | integer | no | Milliseconds to wait between retries. Default: `250`. |

## Model types

### http

Calls an HTTP endpoint for each example. The `request_template` is a JSON string with `{{input}}` substituted for the example input. The `response_path` is a dot-notation path into the response JSON to extract the model output.

```yaml
model:
  type: http
  endpoint: "https://api.openai.com/v1/chat/completions"
  api_key_env: "OPENAI_API_KEY"
  headers:
    Content-Type: "application/json"
    X-Custom-Header: "value"
  request_template: |
    {
      "model": "gpt-4o-mini",
      "messages": [{"role": "user", "content": "{{input}}"}]
    }
  response_path: "choices[0].message.content"
  method: POST   # default: POST
```

The `api_key_env` field specifies the name of an environment variable whose value is sent as a `Bearer` token in the `Authorization` header. You can also set the header manually in `headers`.

### command

Runs a subprocess for each example. The model output is read from stdout.

```yaml
model:
  type: command
  command: ["python", "scripts/run_model.py"]
  input_via: stdin   # or "arg" (appends input as the last arg), or "env" (sets INPUT env var)
  timeout_seconds: 60
```

Useful for wrapping local models, scripts, or tools that aren't exposed over HTTP.

### echo

Returns the input unchanged. Useful for testing graders without a real model.

```yaml
model:
  type: echo
```

### noop

Returns an empty string for every input. Useful for verifying threshold behavior.

```yaml
model:
  type: noop
```

## The Model interface (Go)

When using the Go API, implement the `Model` interface to provide any model behavior:

```go
type Model interface {
    Run(ctx context.Context, input string) (string, error)
}
```

The simplest way to implement it is with `matchspec.ModelFunc`:

```go
model := matchspec.ModelFunc(func(ctx context.Context, input string) (string, error) {
    // Call your model here.
    resp, err := client.Complete(ctx, input)
    if err != nil {
        return "", err
    }
    return resp.Text, nil
})
```

For models that need initialization or cleanup, implement the full interface:

```go
type MyModel struct {
    client *llm.Client
}

func (m *MyModel) Run(ctx context.Context, input string) (string, error) {
    return m.client.Complete(ctx, input)
}
```

## Harness configuration in Go

```go
harness := matchspec.Harness{
    Name:        "summarization-v2",
    Description: "Summarization quality eval.",
    Dataset:     myDataset,
    Model:       myModel,
    Graders: []matchspec.Grader{
        matchspec.NewSemanticSimilarityGrader(matchspec.SemanticSimilarityConfig{
            EmbeddingEndpoint: "https://api.openai.com/v1/embeddings",
            Model:             "text-embedding-3-small",
            APIKey:            os.Getenv("OPENAI_API_KEY"),
            Threshold:         0.82,
        }),
        matchspec.NewExactMatchGrader(matchspec.ExactMatchConfig{
            Threshold:       0.60,
            TrimWhitespace:  true,
        }),
    },
    Concurrency:    8,
    TimeoutSeconds: 30,
    Retries:        2,
}
```

## Concurrency

matchspec runs model calls concurrently within a harness. The `concurrency` field controls how many model calls are in flight at once. The default is 4.

Setting concurrency higher speeds up eval runs but increases load on the model endpoint. For rate-limited APIs (like OpenAI), set concurrency to match your allowed request rate. A concurrency of 8 with a 500ms average latency gives roughly 16 requests per second.

Grader calls run concurrently with model calls — while one batch of model outputs is being scored, the next batch of model calls is already in flight.

## Retry behavior

When a model call fails (network error, timeout, non-2xx response), matchspec can retry it before recording a failure. Configure with `retries` and `retry_delay_ms`:

```yaml
retries: 3
retry_delay_ms: 1000
```

Retries use exponential backoff: the actual delay before retry N is `retry_delay_ms * 2^(N-1)`. With `retry_delay_ms: 1000` and `retries: 3`, the delays are 1s, 2s, and 4s.

If all retries fail, the example is recorded with a `model_error` status and excluded from the pass rate calculation. The number of model errors is reported separately:

```
suite: summarization-v2
─────────────────────────────────────────────
semantic_similarity  0.84  ✓  (≥0.80)
exact_match          0.63  ✓  (≥0.60)
─────────────────────────────────────────────
overall              PASS
model_errors         2 of 120 examples failed
```

## Inline datasets

For small datasets, define examples inline in the harness file:

```yaml
dataset:
  name: quick-check
  examples:
    - id: ex-001
      input: "What is the capital of France?"
      expected: "Paris"
    - id: ex-002
      input: "What is 2 + 2?"
      expected: "4"
```

## Multiple harnesses per suite

A suite can run multiple harnesses and aggregate the results:

```yaml
# matchspec.yml
suites:
  - name: full-eval
    harnesses:
      - ./evals/summarization/harness.yml
      - ./evals/qa/harness.yml
      - ./evals/classification/harness.yml
    thresholds:
      overall: 0.80
```

Each harness runs independently. The suite-level threshold applies to the aggregate pass rate across all harnesses combined. Per-grader thresholds in individual harness files still apply within each harness.
