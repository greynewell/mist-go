package circuitbreaker

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestStressConcurrentCalls(t *testing.T) {
	cb := New(Config{
		Threshold:   100,
		Timeout:     time.Second,
		HalfOpenMax: 1,
	})

	var successes, failures atomic.Int64
	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			err := cb.Do(context.Background(), func(ctx context.Context) error {
				if i%10 == 0 {
					return fmt.Errorf("fail")
				}
				return nil
			})
			if err == nil {
				successes.Add(1)
			} else {
				failures.Add(1)
			}
		}()
	}

	wg.Wait()
	total := successes.Load() + failures.Load()
	if total != 1000 {
		t.Errorf("total = %d, want 1000", total)
	}
}

func TestStressTripAndRecover(t *testing.T) {
	cb := New(Config{
		Threshold:   5,
		Timeout:     50 * time.Millisecond,
		HalfOpenMax: 1,
	})

	// Phase 1: Trip the breaker.
	for i := 0; i < 5; i++ {
		cb.Do(context.Background(), func(ctx context.Context) error {
			return fmt.Errorf("fail")
		})
	}

	if cb.State() != Open {
		t.Fatalf("state = %v, want Open", cb.State())
	}

	// Phase 2: Wait for half-open, then recover.
	time.Sleep(100 * time.Millisecond)

	err := cb.Do(context.Background(), func(ctx context.Context) error {
		return nil // success
	})

	if err != nil {
		t.Fatalf("recovery call: %v", err)
	}
	if cb.State() != Closed {
		t.Errorf("state = %v, want Closed after recovery", cb.State())
	}

	// Phase 3: Normal operation should work.
	for i := 0; i < 100; i++ {
		err := cb.Do(context.Background(), func(ctx context.Context) error {
			return nil
		})
		if err != nil {
			t.Fatalf("post-recovery call %d: %v", i, err)
		}
	}
}

func TestStressRapidTripRecoverCycles(t *testing.T) {
	cb := New(Config{
		Threshold:   2,
		Timeout:     20 * time.Millisecond,
		HalfOpenMax: 1,
	})

	for cycle := 0; cycle < 20; cycle++ {
		// Trip.
		for i := 0; i < 2; i++ {
			cb.Do(context.Background(), func(ctx context.Context) error {
				return fmt.Errorf("fail")
			})
		}

		// Wait for half-open.
		time.Sleep(30 * time.Millisecond)

		// Recover.
		cb.Do(context.Background(), func(ctx context.Context) error {
			return nil
		})

		if cb.State() != Closed {
			t.Fatalf("cycle %d: state = %v, want Closed", cycle, cb.State())
		}
	}
}

func TestStressConcurrentStateTransitions(t *testing.T) {
	cb := New(Config{
		Threshold:   3,
		Timeout:     10 * time.Millisecond,
		HalfOpenMax: 1,
	})

	var wg sync.WaitGroup
	ctx := context.Background()

	// Hammer it from 50 goroutines with mixed success/failure.
	for i := 0; i < 50; i++ {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				cb.Do(ctx, func(ctx context.Context) error {
					if (i+j)%4 == 0 {
						return fmt.Errorf("fail")
					}
					return nil
				})
				// Occasional sleep to let timeouts expire.
				if j%20 == 0 {
					time.Sleep(15 * time.Millisecond)
				}
			}
		}()
	}

	wg.Wait()
	// Should not have panicked or deadlocked.
	_ = cb.State()
}

func TestStressOpenRejectionPerformance(t *testing.T) {
	cb := New(Config{
		Threshold:   1,
		Timeout:     time.Hour, // stay open
		HalfOpenMax: 1,
	})

	// Trip it.
	cb.Do(context.Background(), func(ctx context.Context) error {
		return fmt.Errorf("fail")
	})

	// Measure rejection throughput.
	start := time.Now()
	const n = 100_000
	for i := 0; i < n; i++ {
		cb.Do(context.Background(), func(ctx context.Context) error {
			return nil
		})
	}
	elapsed := time.Since(start)

	// Should be very fast — no function calls, just state check.
	nsPerOp := elapsed.Nanoseconds() / n
	if nsPerOp > 1000 { // 1µs per rejection is too slow
		t.Errorf("rejection too slow: %dns/op", nsPerOp)
	}
}

func TestStressFallbackUnderLoad(t *testing.T) {
	cb := New(Config{
		Threshold:   1,
		Timeout:     time.Hour,
		HalfOpenMax: 1,
	})

	// Trip it.
	cb.Do(context.Background(), func(ctx context.Context) error {
		return fmt.Errorf("fail")
	})

	var fallbacks atomic.Int64
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cb.DoWithFallback(context.Background(),
				func(ctx context.Context) error { return nil },
				func(ctx context.Context, err error) error {
					fallbacks.Add(1)
					return nil
				},
			)
		}()
	}

	wg.Wait()
	if fallbacks.Load() != 100 {
		t.Errorf("fallbacks = %d, want 100", fallbacks.Load())
	}
}
