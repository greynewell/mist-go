package checkpoint

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func tmpDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "checkpoints")
}

func TestBasicStep(t *testing.T) {
	dir := tmpDir(t)
	cp, err := Open(dir, "run-1")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer cp.Close()

	var called int
	err = cp.Step(context.Background(), "step-a", func(_ context.Context) (any, error) {
		called++
		return "result-a", nil
	})

	if err != nil {
		t.Fatalf("Step: %v", err)
	}
	if called != 1 {
		t.Errorf("called = %d, want 1", called)
	}
	if cp.Result("step-a") != "result-a" {
		t.Errorf("Result = %v", cp.Result("step-a"))
	}
}

func TestStepIdempotent(t *testing.T) {
	dir := tmpDir(t)

	// First run: execute the step.
	cp1, err := Open(dir, "run-2")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	cp1.Step(context.Background(), "download", func(_ context.Context) (any, error) {
		return "data-xyz", nil
	})
	cp1.Close()

	// Second run with same ID: step should be skipped.
	cp2, err := Open(dir, "run-2")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer cp2.Close()

	var called int
	cp2.Step(context.Background(), "download", func(_ context.Context) (any, error) {
		called++
		return "should-not-run", nil
	})

	if called != 0 {
		t.Error("step should have been skipped on resume")
	}
	if !cp2.IsCompleted("download") {
		t.Error("download should be marked completed")
	}
}

func TestResumeFromFailure(t *testing.T) {
	dir := tmpDir(t)

	// First run: step-a succeeds, step-b fails.
	cp1, _ := Open(dir, "run-3")
	cp1.Step(context.Background(), "step-a", func(_ context.Context) (any, error) {
		return "ok", nil
	})
	cp1.Step(context.Background(), "step-b", func(_ context.Context) (any, error) {
		return nil, fmt.Errorf("network error")
	})
	cp1.Close()

	// Second run: step-a should be skipped, step-b should re-execute.
	cp2, _ := Open(dir, "run-3")
	defer cp2.Close()

	var stepACalled, stepBCalled int
	cp2.Step(context.Background(), "step-a", func(_ context.Context) (any, error) {
		stepACalled++
		return "should-skip", nil
	})
	cp2.Step(context.Background(), "step-b", func(_ context.Context) (any, error) {
		stepBCalled++
		return "recovered", nil
	})

	if stepACalled != 0 {
		t.Error("step-a should be skipped")
	}
	if stepBCalled != 1 {
		t.Error("step-b should re-execute")
	}
	if !cp2.IsCompleted("step-b") {
		t.Error("step-b should now be completed")
	}
}

func TestResumeCrashedRunning(t *testing.T) {
	dir := tmpDir(t)

	// Simulate a crash: write "running" record with no completion.
	cp1, _ := Open(dir, "run-crash")
	cp1.append(Record{
		Step:      "transform",
		Status:    StatusRunning,
		Timestamp: time.Now(),
	})
	cp1.Close()

	// Resume: the "running" step should NOT be considered complete.
	cp2, _ := Open(dir, "run-crash")
	defer cp2.Close()

	if cp2.IsCompleted("transform") {
		t.Error("crashed running step should not be marked completed")
	}

	var called int
	cp2.Step(context.Background(), "transform", func(_ context.Context) (any, error) {
		called++
		return "done", nil
	})

	if called != 1 {
		t.Error("transform should re-execute after crash")
	}
}

func TestMultipleSteps(t *testing.T) {
	dir := tmpDir(t)
	cp, _ := Open(dir, "run-multi")
	defer cp.Close()

	steps := []string{"download", "parse", "validate", "upload"}
	for _, s := range steps {
		name := s
		cp.Step(context.Background(), name, func(_ context.Context) (any, error) {
			return name + "-done", nil
		})
	}

	completed := cp.CompletedSteps()
	if len(completed) != 4 {
		t.Errorf("completed = %d, want 4", len(completed))
	}

	for _, s := range steps {
		if !cp.IsCompleted(s) {
			t.Errorf("%s should be completed", s)
		}
	}
}

func TestStepRetrySuccess(t *testing.T) {
	dir := tmpDir(t)
	cp, _ := Open(dir, "run-retry")
	defer cp.Close()

	var attempts int32
	err := cp.StepRetry(context.Background(), "flaky-step", 3, func(_ context.Context) (any, error) {
		n := atomic.AddInt32(&attempts, 1)
		if n < 3 {
			return nil, fmt.Errorf("transient error attempt %d", n)
		}
		return "success", nil
	})

	if err != nil {
		t.Fatalf("StepRetry: %v", err)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("attempts = %d, want 3", atomic.LoadInt32(&attempts))
	}
	if !cp.IsCompleted("flaky-step") {
		t.Error("flaky-step should be completed")
	}
}

func TestStepRetryExhausted(t *testing.T) {
	dir := tmpDir(t)
	cp, _ := Open(dir, "run-retry-fail")
	defer cp.Close()

	err := cp.StepRetry(context.Background(), "always-fail", 2, func(_ context.Context) (any, error) {
		return nil, fmt.Errorf("permanent error")
	})

	if err == nil {
		t.Fatal("expected error after exhausted retries")
	}
	if cp.IsCompleted("always-fail") {
		t.Error("always-fail should not be completed")
	}
}

func TestStepRetryIdempotent(t *testing.T) {
	dir := tmpDir(t)

	// First run: step completes after retry.
	cp1, _ := Open(dir, "run-retry-idem")
	var attempts1 int32
	cp1.StepRetry(context.Background(), "retried", 3, func(_ context.Context) (any, error) {
		n := atomic.AddInt32(&attempts1, 1)
		if n < 2 {
			return nil, fmt.Errorf("fail")
		}
		return "ok", nil
	})
	cp1.Close()

	// Second run: step should be skipped entirely.
	cp2, _ := Open(dir, "run-retry-idem")
	defer cp2.Close()

	var attempts2 int32
	cp2.StepRetry(context.Background(), "retried", 3, func(_ context.Context) (any, error) {
		atomic.AddInt32(&attempts2, 1)
		return "should-not-run", nil
	})

	if atomic.LoadInt32(&attempts2) != 0 {
		t.Error("retried step should be skipped on resume")
	}
}

func TestStepRetryContextCancel(t *testing.T) {
	dir := tmpDir(t)
	cp, _ := Open(dir, "run-cancel")
	defer cp.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := cp.StepRetry(ctx, "cancelled", 5, func(_ context.Context) (any, error) {
		return nil, fmt.Errorf("fail")
	})

	if err == nil {
		t.Error("expected error with cancelled context")
	}
}

func TestReset(t *testing.T) {
	dir := tmpDir(t)
	cp, _ := Open(dir, "run-reset")

	cp.Step(context.Background(), "a", func(_ context.Context) (any, error) {
		return "done", nil
	})

	if err := cp.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	if cp.IsCompleted("a") {
		t.Error("step should not be completed after reset")
	}
	cp.Close()
}

func TestRunID(t *testing.T) {
	dir := tmpDir(t)
	cp, _ := Open(dir, "my-run-id")
	defer cp.Close()

	if cp.RunID() != "my-run-id" {
		t.Errorf("RunID = %q", cp.RunID())
	}
}

func TestCheckpointFilePersistence(t *testing.T) {
	dir := tmpDir(t)
	cp, _ := Open(dir, "persist")
	cp.Step(context.Background(), "s1", func(_ context.Context) (any, error) {
		return 42, nil
	})
	cp.Close()

	// Verify the file exists.
	path := filepath.Join(dir, "persist.jsonl")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("checkpoint file not found: %v", err)
	}
	if info.Size() == 0 {
		t.Error("checkpoint file should not be empty")
	}
}

func TestStepError(t *testing.T) {
	dir := tmpDir(t)
	cp, _ := Open(dir, "run-err")
	defer cp.Close()

	err := cp.Step(context.Background(), "fail", func(_ context.Context) (any, error) {
		return nil, fmt.Errorf("oops")
	})

	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "oops" {
		t.Errorf("error = %q", err.Error())
	}
	if cp.IsCompleted("fail") {
		t.Error("failed step should not be completed")
	}
}

func TestConcurrentSteps(t *testing.T) {
	dir := tmpDir(t)
	cp, _ := Open(dir, "run-concurrent")
	defer cp.Close()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 50; i++ {
			name := fmt.Sprintf("step-%d", i)
			cp.Step(context.Background(), name, func(_ context.Context) (any, error) {
				return i, nil
			})
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}

	if len(cp.CompletedSteps()) != 50 {
		t.Errorf("completed = %d, want 50", len(cp.CompletedSteps()))
	}
}
