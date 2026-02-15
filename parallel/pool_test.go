package parallel

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
)

func TestMap(t *testing.T) {
	p := NewPool(4)
	inputs := []int{1, 2, 3, 4, 5}

	results := Map(context.Background(), p, inputs, func(_ context.Context, n int) (int, error) {
		return n * 2, nil
	})

	for i, r := range results {
		if r.Err != nil {
			t.Fatalf("result[%d]: unexpected error: %v", i, r.Err)
		}
		want := inputs[i] * 2
		if r.Value != want {
			t.Errorf("result[%d] = %d, want %d", i, r.Value, want)
		}
	}
}

func TestMapPreservesOrder(t *testing.T) {
	p := NewPool(2)
	inputs := []int{10, 20, 30, 40, 50}

	results := Map(context.Background(), p, inputs, func(_ context.Context, n int) (string, error) {
		return fmt.Sprintf("item-%d", n), nil
	})

	for i, r := range results {
		want := fmt.Sprintf("item-%d", inputs[i])
		if r.Value != want {
			t.Errorf("result[%d] = %q, want %q", i, r.Value, want)
		}
	}
}

func TestMapCancelledContext(t *testing.T) {
	p := NewPool(1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	inputs := []int{1, 2, 3}
	results := Map(ctx, p, inputs, func(ctx context.Context, n int) (int, error) {
		return n, nil
	})

	for _, r := range results {
		if r.Err == nil {
			continue // some may have started before cancel
		}
		if r.Err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", r.Err)
		}
	}
}

func TestMapWithErrors(t *testing.T) {
	p := NewPool(4)
	inputs := []int{1, 2, 3}

	results := Map(context.Background(), p, inputs, func(_ context.Context, n int) (int, error) {
		if n == 2 {
			return 0, fmt.Errorf("bad input: %d", n)
		}
		return n, nil
	})

	if results[0].Err != nil {
		t.Errorf("result[0] should succeed")
	}
	if results[1].Err == nil {
		t.Errorf("result[1] should fail")
	}
	if results[2].Err != nil {
		t.Errorf("result[2] should succeed")
	}
}

func TestDo(t *testing.T) {
	p := NewPool(2)
	var count atomic.Int64

	err := Do(context.Background(), p, []int{1, 2, 3}, func(_ context.Context, _ int) error {
		count.Add(1)
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count.Load() != 3 {
		t.Errorf("count = %d, want 3", count.Load())
	}
}

func TestDoReturnsFirstError(t *testing.T) {
	p := NewPool(1) // sequential to guarantee order
	err := Do(context.Background(), p, []int{1, 2, 3}, func(_ context.Context, n int) error {
		if n == 2 {
			return fmt.Errorf("fail at %d", n)
		}
		return nil
	})

	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFanOut(t *testing.T) {
	p := NewPool(3)
	fns := []func(context.Context, int) (string, error){
		func(_ context.Context, n int) (string, error) { return fmt.Sprintf("a:%d", n), nil },
		func(_ context.Context, n int) (string, error) { return fmt.Sprintf("b:%d", n), nil },
		func(_ context.Context, n int) (string, error) { return fmt.Sprintf("c:%d", n), nil },
	}

	results := FanOut(context.Background(), p, 42, fns)

	want := []string{"a:42", "b:42", "c:42"}
	for i, r := range results {
		if r.Err != nil {
			t.Fatalf("result[%d]: %v", i, r.Err)
		}
		if r.Value != want[i] {
			t.Errorf("result[%d] = %q, want %q", i, r.Value, want[i])
		}
	}
}

func TestNewPoolMinimumWorkers(t *testing.T) {
	p := NewPool(0)
	if p.workers != 1 {
		t.Errorf("workers = %d, want 1", p.workers)
	}
}

func TestPoolConcurrencyLimit(t *testing.T) {
	p := NewPool(2)
	var running atomic.Int64
	var maxSeen atomic.Int64

	inputs := make([]int, 20)
	for i := range inputs {
		inputs[i] = i
	}

	Map(context.Background(), p, inputs, func(_ context.Context, _ int) (int, error) {
		cur := running.Add(1)
		for {
			old := maxSeen.Load()
			if cur <= old || maxSeen.CompareAndSwap(old, cur) {
				break
			}
		}
		running.Add(-1)
		return 0, nil
	})

	if maxSeen.Load() > 2 {
		t.Errorf("max concurrent = %d, want <= 2", maxSeen.Load())
	}
}
