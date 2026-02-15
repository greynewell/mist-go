package parallel

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestStressPoolHighVolume runs a large number of items through the pool.
func TestStressPoolHighVolume(t *testing.T) {
	p := NewPool(16)
	const count = 100_000

	inputs := make([]int, count)
	for i := range inputs {
		inputs[i] = i
	}

	results := Map(context.Background(), p, inputs, func(_ context.Context, n int) (int, error) {
		return n * 2, nil
	})

	for i, r := range results {
		if r.Err != nil {
			t.Fatalf("result[%d]: %v", i, r.Err)
		}
		if r.Value != i*2 {
			t.Fatalf("result[%d] = %d, want %d", i, r.Value, i*2)
		}
	}
}

// TestStressPoolConcurrencyInvariant verifies that the pool never exceeds
// its concurrency limit under heavy load.
func TestStressPoolConcurrencyInvariant(t *testing.T) {
	const maxWorkers = 8
	p := NewPool(maxWorkers)
	const count = 10_000

	var running atomic.Int64
	var maxSeen atomic.Int64
	var violations atomic.Int64

	inputs := make([]int, count)
	for i := range inputs {
		inputs[i] = i
	}

	Map(context.Background(), p, inputs, func(_ context.Context, _ int) (int, error) {
		cur := running.Add(1)
		if cur > int64(maxWorkers) {
			violations.Add(1)
		}
		for {
			old := maxSeen.Load()
			if cur <= old || maxSeen.CompareAndSwap(old, cur) {
				break
			}
		}
		time.Sleep(time.Microsecond)
		running.Add(-1)
		return 0, nil
	})

	if violations.Load() > 0 {
		t.Errorf("concurrency limit violated %d times, max seen %d (limit %d)",
			violations.Load(), maxSeen.Load(), maxWorkers)
	}
	t.Logf("max concurrent workers seen: %d (limit %d)", maxSeen.Load(), maxWorkers)
}

// TestStressPoolWithErrors mixes successes and failures under load.
func TestStressPoolWithErrors(t *testing.T) {
	p := NewPool(8)
	const count = 10_000

	inputs := make([]int, count)
	for i := range inputs {
		inputs[i] = i
	}

	results := Map(context.Background(), p, inputs, func(_ context.Context, n int) (string, error) {
		if n%7 == 0 {
			return "", fmt.Errorf("fail at %d", n)
		}
		return fmt.Sprintf("ok-%d", n), nil
	})

	var successes, failures int
	for i, r := range results {
		if inputs[i]%7 == 0 {
			if r.Err == nil {
				t.Errorf("result[%d] should have failed", i)
			}
			failures++
		} else {
			if r.Err != nil {
				t.Errorf("result[%d] should have succeeded: %v", i, r.Err)
			}
			expected := fmt.Sprintf("ok-%d", inputs[i])
			if r.Value != expected {
				t.Errorf("result[%d] = %q, want %q", i, r.Value, expected)
			}
			successes++
		}
	}

	t.Logf("successes: %d, failures: %d", successes, failures)
}

// TestStressPoolContextCancellation cancels midway through work.
func TestStressPoolContextCancellation(t *testing.T) {
	p := NewPool(4)
	const count = 1000

	ctx, cancel := context.WithCancel(context.Background())
	var started atomic.Int64

	inputs := make([]int, count)
	for i := range inputs {
		inputs[i] = i
	}

	go func() {
		for started.Load() < 10 {
			time.Sleep(time.Millisecond)
		}
		cancel()
	}()

	results := Map(ctx, p, inputs, func(ctx context.Context, n int) (int, error) {
		started.Add(1)
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(10 * time.Millisecond):
			return n, nil
		}
	})

	var completed, cancelled int
	for _, r := range results {
		if r.Err != nil {
			cancelled++
		} else {
			completed++
		}
	}

	t.Logf("completed: %d, cancelled: %d (of %d)", completed, cancelled, count)
	if cancelled == 0 {
		t.Error("expected some cancellations")
	}
}

// TestStressRateLimiterThroughput verifies the rate limiter stays close
// to the configured rate.
func TestStressRateLimiterThroughput(t *testing.T) {
	const rate = 200
	const duration = 1 * time.Second

	rl := NewRateLimiter(rate, time.Second)

	// Drain the initial bucket so we measure steady-state.
	for i := 0; i < rate; i++ {
		rl.TryTake()
	}

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var count int64
	for {
		if err := rl.Wait(ctx); err != nil {
			break
		}
		count++
	}

	expected := float64(rate) * float64(duration) / float64(time.Second)
	ratio := float64(count) / expected

	t.Logf("rate limiter: %d ops in %v (expected ~%.0f, ratio %.2f)", count, duration, expected, ratio)

	// Allow generous tolerance since timing is imprecise in CI.
	if ratio < 0.3 || ratio > 3.0 {
		t.Errorf("rate limiter ratio %.2f is too far from 1.0", ratio)
	}
}

// TestStressRateLimiterConcurrent verifies rate limiting with concurrent waiters.
func TestStressRateLimiterConcurrent(t *testing.T) {
	const rate = 100
	rl := NewRateLimiter(rate, time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	var total atomic.Int64
	var wg sync.WaitGroup

	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				if err := rl.Wait(ctx); err != nil {
					return
				}
				total.Add(1)
			}
		}()
	}

	wg.Wait()
	t.Logf("concurrent rate limiter: %d total ops with %d goroutines", total.Load(), 10)
}

// TestStressFanOutLargeScale runs FanOut with many functions.
func TestStressFanOutLargeScale(t *testing.T) {
	p := NewPool(16)
	const numFns = 100

	fns := make([]func(context.Context, int) (string, error), numFns)
	for i := range fns {
		idx := i
		fns[i] = func(_ context.Context, n int) (string, error) {
			return fmt.Sprintf("fn%d(%d)", idx, n), nil
		}
	}

	results := FanOut(context.Background(), p, 42, fns)

	for i, r := range results {
		if r.Err != nil {
			t.Fatalf("result[%d]: %v", i, r.Err)
		}
		expected := fmt.Sprintf("fn%d(42)", i)
		if r.Value != expected {
			t.Errorf("result[%d] = %q, want %q", i, r.Value, expected)
		}
	}
}

// TestStressDoHighVolume runs Do with many items and verifies all execute.
func TestStressDoHighVolume(t *testing.T) {
	p := NewPool(16)
	const count = 50_000

	var executed atomic.Int64
	inputs := make([]int, count)
	for i := range inputs {
		inputs[i] = i
	}

	err := Do(context.Background(), p, inputs, func(_ context.Context, _ int) error {
		executed.Add(1)
		return nil
	})

	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if executed.Load() != int64(count) {
		t.Errorf("executed %d, want %d", executed.Load(), count)
	}
}
