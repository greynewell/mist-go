package circuitbreaker

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestClosedPassesThrough(t *testing.T) {
	cb := New(Config{
		Threshold:   5,
		Timeout:     time.Second,
		HalfOpenMax: 1,
	})

	err := cb.Do(context.Background(), func(ctx context.Context) error {
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cb.State() != Closed {
		t.Errorf("state = %v, want Closed", cb.State())
	}
}

func TestOpensAfterThreshold(t *testing.T) {
	cb := New(Config{
		Threshold:   3,
		Timeout:     time.Second,
		HalfOpenMax: 1,
	})

	for i := 0; i < 3; i++ {
		cb.Do(context.Background(), func(ctx context.Context) error {
			return fmt.Errorf("fail %d", i)
		})
	}

	if cb.State() != Open {
		t.Errorf("state = %v, want Open", cb.State())
	}
}

func TestOpenRejectsImmediately(t *testing.T) {
	cb := New(Config{
		Threshold:   1,
		Timeout:     time.Second,
		HalfOpenMax: 1,
	})

	// Trip it.
	cb.Do(context.Background(), func(ctx context.Context) error {
		return fmt.Errorf("fail")
	})

	// Next call should be rejected without calling fn.
	var called bool
	err := cb.Do(context.Background(), func(ctx context.Context) error {
		called = true
		return nil
	})

	if called {
		t.Error("fn should not have been called when open")
	}
	if err != ErrOpen {
		t.Errorf("err = %v, want ErrOpen", err)
	}
}

func TestTransitionsToHalfOpen(t *testing.T) {
	cb := New(Config{
		Threshold:   1,
		Timeout:     50 * time.Millisecond,
		HalfOpenMax: 1,
	})

	// Trip it.
	cb.Do(context.Background(), func(ctx context.Context) error {
		return fmt.Errorf("fail")
	})
	if cb.State() != Open {
		t.Fatalf("state = %v, want Open", cb.State())
	}

	// Wait for timeout.
	time.Sleep(100 * time.Millisecond)

	if cb.State() != HalfOpen {
		t.Errorf("state = %v, want HalfOpen", cb.State())
	}
}

func TestHalfOpenSuccessCloses(t *testing.T) {
	cb := New(Config{
		Threshold:   1,
		Timeout:     50 * time.Millisecond,
		HalfOpenMax: 1,
	})

	// Trip → open.
	cb.Do(context.Background(), func(ctx context.Context) error {
		return fmt.Errorf("fail")
	})

	// Wait → half-open.
	time.Sleep(100 * time.Millisecond)

	// Success in half-open → closed.
	err := cb.Do(context.Background(), func(ctx context.Context) error {
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cb.State() != Closed {
		t.Errorf("state = %v, want Closed", cb.State())
	}
}

func TestHalfOpenFailureReopens(t *testing.T) {
	cb := New(Config{
		Threshold:   1,
		Timeout:     50 * time.Millisecond,
		HalfOpenMax: 1,
	})

	// Trip → open.
	cb.Do(context.Background(), func(ctx context.Context) error {
		return fmt.Errorf("fail")
	})

	// Wait → half-open.
	time.Sleep(100 * time.Millisecond)

	// Fail in half-open → open again.
	cb.Do(context.Background(), func(ctx context.Context) error {
		return fmt.Errorf("still broken")
	})

	if cb.State() != Open {
		t.Errorf("state = %v, want Open", cb.State())
	}
}

func TestHalfOpenLimitsProbes(t *testing.T) {
	cb := New(Config{
		Threshold:   1,
		Timeout:     50 * time.Millisecond,
		HalfOpenMax: 1,
	})

	// Trip → open.
	cb.Do(context.Background(), func(ctx context.Context) error {
		return fmt.Errorf("fail")
	})

	time.Sleep(100 * time.Millisecond)

	// First call in half-open: allowed.
	started := make(chan struct{})
	done := make(chan struct{})
	go func() {
		cb.Do(context.Background(), func(ctx context.Context) error {
			close(started)
			<-done // block
			return nil
		})
	}()
	<-started

	// Second concurrent call in half-open: rejected.
	err := cb.Do(context.Background(), func(ctx context.Context) error {
		return nil
	})
	close(done)

	if err != ErrOpen {
		t.Errorf("concurrent half-open call: err = %v, want ErrOpen", err)
	}
}

func TestCountsResetOnClose(t *testing.T) {
	cb := New(Config{
		Threshold:   3,
		Timeout:     50 * time.Millisecond,
		HalfOpenMax: 1,
	})

	// Accumulate 2 failures (below threshold).
	cb.Do(context.Background(), func(ctx context.Context) error {
		return fmt.Errorf("fail")
	})
	cb.Do(context.Background(), func(ctx context.Context) error {
		return fmt.Errorf("fail")
	})

	// Success resets count.
	cb.Do(context.Background(), func(ctx context.Context) error {
		return nil
	})

	// Need 3 more failures to trip.
	for i := 0; i < 2; i++ {
		cb.Do(context.Background(), func(ctx context.Context) error {
			return fmt.Errorf("fail")
		})
	}

	// Still closed because count was reset.
	if cb.State() != Closed {
		t.Errorf("state = %v, want Closed", cb.State())
	}
}

func TestFallback(t *testing.T) {
	cb := New(Config{
		Threshold:   1,
		Timeout:     time.Second,
		HalfOpenMax: 1,
	})

	// Trip it.
	cb.Do(context.Background(), func(ctx context.Context) error {
		return fmt.Errorf("fail")
	})

	// With fallback.
	var fallbackCalled bool
	err := cb.DoWithFallback(context.Background(),
		func(ctx context.Context) error {
			return nil
		},
		func(ctx context.Context, err error) error {
			fallbackCalled = true
			return nil
		},
	)

	if err != nil {
		t.Fatalf("unexpected error with fallback: %v", err)
	}
	if !fallbackCalled {
		t.Error("fallback should have been called")
	}
}

func TestFallbackReceivesError(t *testing.T) {
	cb := New(Config{
		Threshold:   1,
		Timeout:     time.Second,
		HalfOpenMax: 1,
	})

	// Trip it.
	cb.Do(context.Background(), func(ctx context.Context) error {
		return fmt.Errorf("fail")
	})

	var received error
	cb.DoWithFallback(context.Background(),
		func(ctx context.Context) error { return nil },
		func(ctx context.Context, err error) error {
			received = err
			return nil
		},
	)

	if received != ErrOpen {
		t.Errorf("fallback received = %v, want ErrOpen", received)
	}
}

func TestCounts(t *testing.T) {
	cb := New(Config{
		Threshold:   5,
		Timeout:     time.Second,
		HalfOpenMax: 1,
	})

	cb.Do(context.Background(), func(ctx context.Context) error { return nil })
	cb.Do(context.Background(), func(ctx context.Context) error { return nil })
	cb.Do(context.Background(), func(ctx context.Context) error { return fmt.Errorf("x") })

	s, f := cb.Counts()
	if s != 2 {
		t.Errorf("successes = %d, want 2", s)
	}
	if f != 1 {
		t.Errorf("failures = %d, want 1", f)
	}
}

func TestContextCancellation(t *testing.T) {
	cb := New(Config{
		Threshold:   5,
		Timeout:     time.Second,
		HalfOpenMax: 1,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := cb.Do(ctx, func(ctx context.Context) error {
		return ctx.Err()
	})

	if err != context.Canceled {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	// Cancelled context errors should not count toward tripping.
	_, f := cb.Counts()
	if f != 0 {
		t.Errorf("failures = %d, want 0 (context errors don't trip)", f)
	}
}
