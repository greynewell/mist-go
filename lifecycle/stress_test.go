package lifecycle

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestStressConcurrentShutdownHooks(t *testing.T) {
	var hookCount atomic.Int32

	err := Run(func(ctx context.Context) error {
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				OnShutdown(ctx, func() error {
					hookCount.Add(1)
					return nil
				})
			}()
		}
		wg.Wait()
		return nil
	})

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if hookCount.Load() != 100 {
		t.Errorf("hooks = %d, want 100", hookCount.Load())
	}
}

func TestStressConcurrentDrainGroups(t *testing.T) {
	var completed atomic.Int32

	err := Run(func(ctx context.Context) error {
		for i := 0; i < 50; i++ {
			dg := DrainGroup(ctx)
			dg.Add(1)
			go func() {
				time.Sleep(time.Duration(1+completed.Load()) * time.Millisecond)
				completed.Add(1)
				dg.Done()
			}()
		}
		return nil
	}, WithDrainTimeout(5*time.Second))

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if completed.Load() != 50 {
		t.Errorf("completed = %d, want 50", completed.Load())
	}
}

func TestStressRapidRunCycles(t *testing.T) {
	for i := 0; i < 100; i++ {
		err := Run(func(ctx context.Context) error {
			OnShutdown(ctx, func() error { return nil })
			dg := DrainGroup(ctx)
			dg.Add(1)
			go func() {
				dg.Done()
			}()
			return nil
		})
		if err != nil {
			t.Fatalf("cycle %d: %v", i, err)
		}
	}
}

func TestStressHookErrors(t *testing.T) {
	err := Run(func(ctx context.Context) error {
		for i := 0; i < 20; i++ {
			i := i
			OnShutdown(ctx, func() error {
				if i%3 == 0 {
					return fmt.Errorf("hook %d failed", i)
				}
				return nil
			})
		}
		return nil
	})

	if err == nil {
		t.Fatal("expected at least one hook error")
	}
}

func TestStressDrainAndShutdownOrdering(t *testing.T) {
	// Verify drain always completes before shutdown hooks, even under concurrency.
	for i := 0; i < 50; i++ {
		var drainDone atomic.Bool
		var hookSawDrain atomic.Bool

		Run(func(ctx context.Context) error {
			dg := DrainGroup(ctx)
			dg.Add(1)

			go func() {
				time.Sleep(5 * time.Millisecond)
				drainDone.Store(true)
				dg.Done()
			}()

			OnShutdown(ctx, func() error {
				hookSawDrain.Store(drainDone.Load())
				return nil
			})

			return nil
		})

		if !hookSawDrain.Load() {
			t.Fatalf("iteration %d: shutdown hook ran before drain completed", i)
		}
	}
}
