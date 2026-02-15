package retry

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	misterrors "github.com/greynewell/mist-go/errors"
)

func TestDoSuccess(t *testing.T) {
	var calls int
	err := Do(context.Background(), DefaultPolicy, func(_ context.Context) error {
		calls++
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestDoRetryThenSuccess(t *testing.T) {
	var calls int
	err := Do(context.Background(), Policy{
		MaxAttempts: 3,
		InitialWait: time.Millisecond,
		Multiplier:  1.0,
	}, func(_ context.Context) error {
		calls++
		if calls < 3 {
			return fmt.Errorf("transient error")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestDoAllFail(t *testing.T) {
	var calls int
	err := Do(context.Background(), Policy{
		MaxAttempts: 3,
		InitialWait: time.Millisecond,
		Multiplier:  1.0,
	}, func(_ context.Context) error {
		calls++
		return fmt.Errorf("fail %d", calls)
	})

	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
	if err.Error() != "fail 3" {
		t.Errorf("error = %q, want last error", err.Error())
	}
}

func TestDoContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var calls int
	err := Do(ctx, Policy{
		MaxAttempts: 5,
		InitialWait: time.Second,
		Multiplier:  2.0,
	}, func(_ context.Context) error {
		calls++
		return fmt.Errorf("fail")
	})

	if err == nil {
		t.Fatal("expected error")
	}
	// Should have bailed quickly.
	if calls > 2 {
		t.Errorf("calls = %d, expected <= 2 with cancelled context", calls)
	}
}

func TestDoContextCancelledBeforeFirstAttempt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Do(ctx, DefaultPolicy, func(_ context.Context) error {
		return nil // would succeed if reached
	})

	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestDoWithClassifier(t *testing.T) {
	retryable := func(err error) bool {
		return err.Error() == "transient"
	}

	var calls int
	err := DoWithClassifier(context.Background(), Policy{
		MaxAttempts: 5,
		InitialWait: time.Millisecond,
		Multiplier:  1.0,
	}, retryable, func(_ context.Context) error {
		calls++
		if calls == 1 {
			return fmt.Errorf("transient")
		}
		return fmt.Errorf("permanent")
	})

	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2 (stop at non-retryable)", calls)
	}
	if err.Error() != "permanent" {
		t.Errorf("error = %q, want permanent", err.Error())
	}
}

func TestDoBackoffTiming(t *testing.T) {
	start := time.Now()
	var calls int

	Do(context.Background(), Policy{
		MaxAttempts: 3,
		InitialWait: 50 * time.Millisecond,
		MaxWait:     200 * time.Millisecond,
		Multiplier:  2.0,
		Jitter:      0,
	}, func(_ context.Context) error {
		calls++
		return fmt.Errorf("fail")
	})

	elapsed := time.Since(start)
	// Expected: 50ms + 100ms = 150ms minimum.
	if elapsed < 100*time.Millisecond {
		t.Errorf("elapsed = %v, expected >= 100ms", elapsed)
	}
}

func TestDoMaxWaitCap(t *testing.T) {
	start := time.Now()

	Do(context.Background(), Policy{
		MaxAttempts: 4,
		InitialWait: 100 * time.Millisecond,
		MaxWait:     100 * time.Millisecond,
		Multiplier:  10.0,
		Jitter:      0,
	}, func(_ context.Context) error {
		return fmt.Errorf("fail")
	})

	elapsed := time.Since(start)
	// 3 waits of 100ms each (capped) = 300ms.
	if elapsed > 500*time.Millisecond {
		t.Errorf("elapsed = %v, max wait should be capped", elapsed)
	}
}

func TestDoMinAttempts(t *testing.T) {
	var calls int
	Do(context.Background(), Policy{MaxAttempts: 0}, func(_ context.Context) error {
		calls++
		return nil
	})
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (min 1 attempt)", calls)
	}
}

func TestPolicyTotalMaxWait(t *testing.T) {
	p := Policy{
		MaxAttempts: 4,
		InitialWait: 100 * time.Millisecond,
		MaxWait:     1 * time.Second,
		Multiplier:  2.0,
	}

	total := p.TotalMaxWait()
	// 100ms + 200ms + 400ms = 700ms
	if total != 700*time.Millisecond {
		t.Errorf("TotalMaxWait = %v, want 700ms", total)
	}
}

func TestDoConcurrent(t *testing.T) {
	var total atomic.Int64
	var wg atomic.Int64

	ctx := context.Background()
	p := Policy{
		MaxAttempts: 2,
		InitialWait: time.Millisecond,
		Multiplier:  1.0,
	}

	done := make(chan struct{})
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Add(-1)
			Do(ctx, p, func(_ context.Context) error {
				total.Add(1)
				return nil
			})
		}()
	}

	// Wait for all goroutines.
	go func() {
		for wg.Load() > 0 {
			time.Sleep(time.Millisecond)
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for concurrent retries")
	}

	if total.Load() != 50 {
		t.Errorf("total = %d, want 50", total.Load())
	}
}

func TestDoAutoRetriesTransient(t *testing.T) {
	var calls int
	err := DoAuto(context.Background(), Policy{
		MaxAttempts: 3,
		InitialWait: time.Millisecond,
		Multiplier:  1.0,
	}, func(_ context.Context) error {
		calls++
		if calls < 3 {
			return misterrors.New(misterrors.CodeUnavailable, "service down")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestDoAutoStopsOnPermanent(t *testing.T) {
	var calls int
	err := DoAuto(context.Background(), Policy{
		MaxAttempts: 5,
		InitialWait: time.Millisecond,
		Multiplier:  1.0,
	}, func(_ context.Context) error {
		calls++
		return misterrors.New(misterrors.CodeValidation, "bad input")
	})

	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (stop on permanent error)", calls)
	}
}
