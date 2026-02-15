package lifecycle

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestRunCallsMain(t *testing.T) {
	var called bool
	err := Run(func(ctx context.Context) error {
		called = true
		return nil
	})

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !called {
		t.Error("main function was not called")
	}
}

func TestRunReturnsMainError(t *testing.T) {
	want := fmt.Errorf("main failed")
	err := Run(func(ctx context.Context) error {
		return want
	})

	if err != want {
		t.Errorf("err = %v, want %v", err, want)
	}
}

func TestRunContextCancelledOnReturn(t *testing.T) {
	cancelled := make(chan error, 1)
	err := Run(func(ctx context.Context) error {
		// Context should be live while main is running.
		if ctx.Err() != nil {
			t.Error("context should not be cancelled yet")
		}
		go func() {
			<-ctx.Done()
			cancelled <- ctx.Err()
		}()
		return nil
	})

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	select {
	case ctxErr := <-cancelled:
		if ctxErr != context.Canceled {
			t.Errorf("ctx err = %v, want context.Canceled", ctxErr)
		}
	case <-time.After(time.Second):
		t.Error("context was not cancelled after Run returned")
	}
}

func TestOnShutdownHooksRun(t *testing.T) {
	var order []int
	var mu sync.Mutex

	err := Run(func(ctx context.Context) error {
		OnShutdown(ctx, func() error {
			mu.Lock()
			order = append(order, 1)
			mu.Unlock()
			return nil
		})
		OnShutdown(ctx, func() error {
			mu.Lock()
			order = append(order, 2)
			mu.Unlock()
			return nil
		})
		return nil
	})

	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Both hooks should have run.
	mu.Lock()
	defer mu.Unlock()
	if len(order) != 2 {
		t.Errorf("hooks run = %d, want 2", len(order))
	}
}

func TestOnShutdownRunsInReverseOrder(t *testing.T) {
	var order []int
	var mu sync.Mutex

	err := Run(func(ctx context.Context) error {
		OnShutdown(ctx, func() error {
			mu.Lock()
			order = append(order, 1)
			mu.Unlock()
			return nil
		})
		OnShutdown(ctx, func() error {
			mu.Lock()
			order = append(order, 2)
			mu.Unlock()
			return nil
		})
		OnShutdown(ctx, func() error {
			mu.Lock()
			order = append(order, 3)
			mu.Unlock()
			return nil
		})
		return nil
	})

	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	// Reverse order: last registered runs first (LIFO, like defer).
	if len(order) != 3 || order[0] != 3 || order[1] != 2 || order[2] != 1 {
		t.Errorf("order = %v, want [3 2 1]", order)
	}
}

func TestOnShutdownErrorCollected(t *testing.T) {
	err := Run(func(ctx context.Context) error {
		OnShutdown(ctx, func() error {
			return fmt.Errorf("cleanup failed")
		})
		return nil
	})

	if err == nil {
		t.Fatal("expected shutdown error")
	}
}

func TestDrainGroupCompletesBeforeShutdown(t *testing.T) {
	var drainCompleted atomic.Bool
	var shutdownSawDrain atomic.Bool

	err := Run(func(ctx context.Context) error {
		dg := DrainGroup(ctx)

		// Start some "in-flight" work.
		dg.Add(1)
		go func() {
			time.Sleep(50 * time.Millisecond)
			drainCompleted.Store(true)
			dg.Done()
		}()

		OnShutdown(ctx, func() error {
			shutdownSawDrain.Store(drainCompleted.Load())
			return nil
		})

		return nil
	})

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !drainCompleted.Load() {
		t.Error("drain should have completed")
	}
	if !shutdownSawDrain.Load() {
		t.Error("shutdown hook should run AFTER drain completes")
	}
}

func TestDrainGroupTimeout(t *testing.T) {
	err := Run(func(ctx context.Context) error {
		dg := DrainGroup(ctx)

		// Start work that will never finish.
		dg.Add(1)
		// Intentionally don't call dg.Done()

		return nil
	}, WithDrainTimeout(100*time.Millisecond))

	// Should still complete (timeout forces shutdown).
	if err == nil {
		t.Fatal("expected timeout error from stuck drain")
	}
}

func TestWithShutdownTimeout(t *testing.T) {
	err := Run(func(ctx context.Context) error {
		OnShutdown(ctx, func() error {
			time.Sleep(5 * time.Second) // Hook that takes too long.
			return nil
		})
		return nil
	}, WithShutdownTimeout(100*time.Millisecond))

	if err == nil {
		t.Fatal("expected timeout error from slow hook")
	}
}

func TestSignalTriggersShutdown(t *testing.T) {
	started := make(chan struct{})
	var ctxCancelled atomic.Bool

	go func() {
		Run(func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			ctxCancelled.Store(true)
			return nil
		})
	}()

	<-started
	// Send ourselves SIGINT.
	time.Sleep(50 * time.Millisecond)
	syscall.Kill(syscall.Getpid(), syscall.SIGINT)
	time.Sleep(200 * time.Millisecond)

	if !ctxCancelled.Load() {
		t.Error("signal should have cancelled context")
	}
}

func TestMultipleDrainGroups(t *testing.T) {
	var completed atomic.Int32

	err := Run(func(ctx context.Context) error {
		dg1 := DrainGroup(ctx)
		dg2 := DrainGroup(ctx)

		dg1.Add(1)
		go func() {
			time.Sleep(20 * time.Millisecond)
			completed.Add(1)
			dg1.Done()
		}()

		dg2.Add(1)
		go func() {
			time.Sleep(30 * time.Millisecond)
			completed.Add(1)
			dg2.Done()
		}()

		return nil
	})

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if completed.Load() != 2 {
		t.Errorf("completed = %d, want 2", completed.Load())
	}
}

func TestRunPanicsRecovered(t *testing.T) {
	err := Run(func(ctx context.Context) error {
		panic("test panic")
	})

	if err == nil {
		t.Fatal("expected error from panic")
	}
}
