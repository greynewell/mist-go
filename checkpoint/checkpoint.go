// Package checkpoint provides incremental checkpointing for long-running
// MIST jobs. Each job gets a unique run ID. Steps within a job are recorded
// to a local JSON-lines file so that if the process dies and restarts with
// the same run ID, already-completed steps are skipped automatically.
//
// Usage:
//
//	cp, err := checkpoint.Open("/tmp/my-job", "run-abc-123")
//	defer cp.Close()
//
//	// Steps that already completed on a previous run are skipped.
//	cp.Step("download", func(ctx context.Context) (any, error) {
//	    return downloadData(ctx)
//	})
//	cp.Step("process", func(ctx context.Context) (any, error) {
//	    return processData(ctx)
//	})
package checkpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Status represents the state of a step.
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusSkipped   Status = "skipped"
)

// Record is a single checkpoint entry persisted to the log file.
type Record struct {
	Step      string    `json:"step"`
	Status    Status    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Result    any       `json:"result,omitempty"`
	Error     string    `json:"error,omitempty"`
	Attempt   int       `json:"attempt,omitempty"`
}

// Tracker manages checkpoint state for a single job run.
type Tracker struct {
	runID     string
	dir       string
	mu        sync.Mutex
	file      *os.File
	completed map[string]*Record
	results   map[string]any
}

// Open creates or resumes a checkpoint tracker. The dir is the base directory
// for checkpoint files. The runID uniquely identifies this job execution —
// reusing the same runID resumes from the last successful step.
func Open(dir, runID string) (*Tracker, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("checkpoint: mkdir %s: %w", dir, err)
	}

	path := filepath.Join(dir, runID+".jsonl")
	t := &Tracker{
		runID:     runID,
		dir:       dir,
		completed: make(map[string]*Record),
		results:   make(map[string]any),
	}

	// Replay existing checkpoint log.
	if data, err := os.ReadFile(path); err == nil {
		t.replay(data)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("checkpoint: open %s: %w", path, err)
	}
	t.file = f

	return t, nil
}

// replay parses existing checkpoint records and rebuilds state.
func (t *Tracker) replay(data []byte) {
	dec := json.NewDecoder(jsonlReader(data))
	for dec.More() {
		var r Record
		if err := dec.Decode(&r); err != nil {
			continue // skip corrupted lines
		}
		switch r.Status {
		case StatusCompleted:
			t.completed[r.Step] = &r
			t.results[r.Step] = r.Result
		case StatusFailed, StatusRunning:
			// A step that was running when the process died needs re-execution.
			delete(t.completed, r.Step)
			delete(t.results, r.Step)
		}
	}
}

// Step executes fn if the step has not already completed in a previous run.
// If the step was already completed, fn is not called and the previous
// result is available via Result(). Returns any error from fn.
func (t *Tracker) Step(ctx context.Context, name string, fn func(ctx context.Context) (any, error)) error {
	t.mu.Lock()
	if _, done := t.completed[name]; done {
		t.mu.Unlock()
		return nil
	}
	t.mu.Unlock()

	// Record that we're starting.
	t.append(Record{
		Step:      name,
		Status:    StatusRunning,
		Timestamp: time.Now(),
	})

	result, err := fn(ctx)
	if err != nil {
		t.append(Record{
			Step:      name,
			Status:    StatusFailed,
			Timestamp: time.Now(),
			Error:     err.Error(),
		})
		return err
	}

	r := Record{
		Step:      name,
		Status:    StatusCompleted,
		Timestamp: time.Now(),
		Result:    result,
	}
	t.append(r)

	t.mu.Lock()
	t.completed[name] = &r
	t.results[name] = result
	t.mu.Unlock()

	return nil
}

// StepRetry executes fn with up to maxAttempts retries using the retry
// package's logic. Each attempt is logged. The step is skipped if already
// completed from a previous run.
func (t *Tracker) StepRetry(ctx context.Context, name string, maxAttempts int, fn func(ctx context.Context) (any, error)) error {
	t.mu.Lock()
	if _, done := t.completed[name]; done {
		t.mu.Unlock()
		return nil
	}
	t.mu.Unlock()

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		t.append(Record{
			Step:      name,
			Status:    StatusRunning,
			Timestamp: time.Now(),
			Attempt:   attempt,
		})

		result, err := fn(ctx)
		if err == nil {
			r := Record{
				Step:      name,
				Status:    StatusCompleted,
				Timestamp: time.Now(),
				Result:    result,
				Attempt:   attempt,
			}
			t.append(r)

			t.mu.Lock()
			t.completed[name] = &r
			t.results[name] = result
			t.mu.Unlock()

			return nil
		}

		lastErr = err
		t.append(Record{
			Step:      name,
			Status:    StatusFailed,
			Timestamp: time.Now(),
			Error:     err.Error(),
			Attempt:   attempt,
		})

		// Exponential backoff: 100ms, 200ms, 400ms, ...
		wait := time.Duration(1<<uint(attempt-1)) * 100 * time.Millisecond
		if wait > 5*time.Second {
			wait = 5 * time.Second
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}

	return lastErr
}

// IsCompleted reports whether the named step has already completed.
func (t *Tracker) IsCompleted(name string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, ok := t.completed[name]
	return ok
}

// Result returns the stored result for a completed step. Returns nil
// if the step hasn't completed.
func (t *Tracker) Result(name string) any {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.results[name]
}

// CompletedSteps returns the names of all completed steps.
func (t *Tracker) CompletedSteps() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	steps := make([]string, 0, len(t.completed))
	for k := range t.completed {
		steps = append(steps, k)
	}
	return steps
}

// RunID returns the run identifier.
func (t *Tracker) RunID() string {
	return t.runID
}

// Close flushes and closes the checkpoint file.
func (t *Tracker) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.file != nil {
		return t.file.Close()
	}
	return nil
}

// Reset deletes the checkpoint file, forcing a full re-run next time.
func (t *Tracker) Reset() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.completed = make(map[string]*Record)
	t.results = make(map[string]any)
	path := filepath.Join(t.dir, t.runID+".jsonl")
	return os.Remove(path)
}

// append writes a record to the checkpoint file.
func (t *Tracker) append(r Record) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.file == nil {
		return
	}
	data, err := json.Marshal(r)
	if err != nil {
		return
	}
	data = append(data, '\n')
	t.file.Write(data)
	t.file.Sync() // fsync for durability
}

// jsonlReader wraps raw bytes for use with json.Decoder so it handles
// newline-delimited JSON.
type jsonlReaderType struct {
	data []byte
	pos  int
}

func jsonlReader(data []byte) *jsonlReaderType {
	return &jsonlReaderType{data: data}
}

func (r *jsonlReaderType) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, fmt.Errorf("EOF")
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
