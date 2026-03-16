---
title: checkpoint
description: The checkpoint package — Tracker, Step, StepRetry, run IDs, JSON-lines persistence, resuming interrupted jobs, and concurrent access.
---

# checkpoint

Import path: `github.com/greynewell/mist-go/checkpoint`

The `checkpoint` package provides incremental checkpointing for long-running MIST jobs. Each job has a unique run ID. Steps within the job are recorded to a local JSON-lines file. If the process dies and restarts with the same run ID, steps that already completed are skipped automatically. This makes long eval runs and agent jobs resumable without re-doing completed work.

## Opening a tracker

```go
cp, err := checkpoint.Open("/var/lib/matchspec/checkpoints", "swe-bench-2026-03-15-run-001")
if err != nil {
    return fmt.Errorf("checkpoint: %w", err)
}
defer cp.Close()
```

`Open` takes a directory and a run ID. The run ID must contain only alphanumeric characters, hyphens, and underscores (enforced by `ValidRunID`). The directory is created if it does not exist.

If a checkpoint file for the run ID already exists (from a previous run), it is replayed: completed steps are loaded into memory, and the file is reopened for appending. Resuming a run requires only that the same run ID and directory are used.

## Registering steps

`Step` executes a function if it has not already completed in a previous run. If the step already completed, the function is not called:

```go
// Step 1: fetch the dataset.
err = cp.Step(ctx, "fetch-dataset", func(ctx context.Context) (any, error) {
    return fetchDataset(ctx, cfg.DatasetURL)
})
if err != nil {
    return err
}

// Step 2: run inference on all examples.
err = cp.Step(ctx, "run-inference", func(ctx context.Context) (any, error) {
    dataset := cp.Result("fetch-dataset").([]Example)
    return runInference(ctx, dataset)
})
if err != nil {
    return err
}

// Step 3: score results.
err = cp.Step(ctx, "score-results", func(ctx context.Context) (any, error) {
    results := cp.Result("run-inference").([]InferResult)
    return scoreResults(ctx, results)
})
```

If the process crashes after "run-inference" completes but before "score-results" does, the next run will:
1. Skip "fetch-dataset" (already completed)
2. Skip "run-inference" (already completed)
3. Execute "score-results" (not yet completed)

The `any` return value from each step is persisted to the checkpoint file and available via `Result(name)` in subsequent steps.

## Accessing previous results

```go
// Get the result from a previous step.
// Returns nil if the step hasn't completed.
raw := cp.Result("fetch-dataset")
if raw == nil {
    return fmt.Errorf("fetch-dataset not completed")
}
dataset, ok := raw.([]Example)
```

Because results are serialized and deserialized as `any` via JSON, type assertions against complex types may require a `json.Unmarshal` step. For robustness, prefer serializing to a concrete type:

```go
err = cp.Step(ctx, "fetch-dataset", func(ctx context.Context) (any, error) {
    examples, err := fetchDataset(ctx, cfg.DatasetURL)
    if err != nil {
        return nil, err
    }
    // Return a JSON-safe value for clean deserialization.
    return examples, nil
})

// In the next step, unmarshal from the intermediate JSON representation.
raw := cp.Result("fetch-dataset")
data, _ := json.Marshal(raw)
var examples []Example
json.Unmarshal(data, &examples)
```

## Retrying steps

`StepRetry` combines checkpointing with built-in retry logic. It retries up to `maxAttempts` times with exponential backoff (100ms, 200ms, 400ms..., capped at 5s):

```go
err = cp.StepRetry(ctx, "run-inference", 3, func(ctx context.Context) (any, error) {
    return callModelWithRetry(ctx, examples)
})
```

Each attempt is logged to the checkpoint file. If all attempts fail, the step is recorded as failed, and the next resume will retry it again from attempt 1.

## Checking step state

```go
// Is this step already done from a previous run?
if cp.IsCompleted("fetch-dataset") {
    log.Printf("skipping fetch-dataset (already done)")
}

// List all completed steps.
steps := cp.CompletedSteps()
// []string{"fetch-dataset", "run-inference"}
```

## Persistent state format

Each step event is written as a JSON line to `{dir}/{runID}.jsonl`:

```json
{"step":"fetch-dataset","status":"running","timestamp":"2026-03-15T10:00:00Z"}
{"step":"fetch-dataset","status":"completed","timestamp":"2026-03-15T10:00:01Z","result":[...]}
{"step":"run-inference","status":"running","timestamp":"2026-03-15T10:00:01Z"}
{"step":"run-inference","status":"failed","timestamp":"2026-03-15T10:00:05Z","error":"connection refused"}
{"step":"run-inference","status":"running","timestamp":"2026-03-15T10:00:05Z","attempt":2}
{"step":"run-inference","status":"completed","timestamp":"2026-03-15T10:00:30Z","attempt":2,"result":[...]}
```

Step statuses: `pending`, `running`, `completed`, `failed`, `skipped`.

The file is opened in append mode and `fsync`'d after each write for durability. On replay, the last recorded status wins: a step that was `running` when the process died is treated as incomplete and will re-execute.

## Resetting a run

To force a complete re-run (delete the checkpoint file):

```go
if err := cp.Reset(); err != nil {
    return fmt.Errorf("checkpoint reset: %w", err)
}
```

`Reset` removes the checkpoint file and clears in-memory state. The next call to `Step` will execute every step from scratch.

## Run ID

The run ID is available via `RunID()`:

```go
fmt.Println(cp.RunID()) // "swe-bench-2026-03-15-run-001"
```

For automated pipelines, generate run IDs from a date, a content hash, or a random suffix:

```go
runID := fmt.Sprintf("swe-bench-%s-%s",
    time.Now().Format("2006-01-02"),
    shortHash(cfg.DatasetURL),
)
```

## Concurrent access

The `Tracker` is safe for concurrent use. Multiple goroutines can call `Step`, `Result`, and `IsCompleted` concurrently. However, two goroutines calling `Step` with the same step name will both execute the function — there is no distributed locking. For concurrent fan-out over a set of tasks, use distinct step names:

```go
pool := parallel.NewPool(8)
results := parallel.Map(ctx, pool, examples, func(ctx context.Context, ex Example) (Result, error) {
    var result Result
    err := cp.Step(ctx, "example-"+ex.ID, func(ctx context.Context) (any, error) {
        return runExample(ctx, ex)
    })
    if err != nil {
        return Result{}, err
    }
    raw := cp.Result("example-" + ex.ID)
    result = raw.(Result)
    return result, nil
})
```
