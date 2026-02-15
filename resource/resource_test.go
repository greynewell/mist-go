package resource

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Limiter tests

func TestLimiterAcquireRelease(t *testing.T) {
	l := NewLimiter("test", 3)

	if err := l.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	if l.Active() != 1 {
		t.Errorf("active = %d, want 1", l.Active())
	}
	if l.Total() != 1 {
		t.Errorf("total = %d, want 1", l.Total())
	}

	l.Release()
	if l.Active() != 0 {
		t.Errorf("active after release = %d, want 0", l.Active())
	}
	if l.Total() != 1 {
		t.Errorf("total should still be 1, got %d", l.Total())
	}
}

func TestLimiterBlocks(t *testing.T) {
	l := NewLimiter("test", 1)
	l.Acquire(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := l.Acquire(ctx)
	if err == nil {
		t.Error("expected acquire to block and timeout")
	}

	l.Release()
}

func TestLimiterTryAcquire(t *testing.T) {
	l := NewLimiter("test", 1)

	if !l.TryAcquire() {
		t.Error("first TryAcquire should succeed")
	}

	if l.TryAcquire() {
		t.Error("second TryAcquire should fail (full)")
	}

	l.Release()

	if !l.TryAcquire() {
		t.Error("TryAcquire after release should succeed")
	}
	l.Release()
}

func TestLimiterGo(t *testing.T) {
	l := NewLimiter("test", 2)
	done := make(chan struct{})

	err := l.Go(context.Background(), func() {
		close(done)
	})
	if err != nil {
		t.Fatal(err)
	}

	<-done
	// Wait for Release to complete.
	time.Sleep(10 * time.Millisecond)

	if l.Active() != 0 {
		t.Errorf("active = %d, want 0 after goroutine completes", l.Active())
	}
}

func TestLimiterGoBlocked(t *testing.T) {
	l := NewLimiter("test", 1)
	l.Acquire(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := l.Go(ctx, func() {
		t.Error("should not run")
	})
	if err == nil {
		t.Error("expected error when limiter is full")
	}

	l.Release()
}

func TestLimiterMinOne(t *testing.T) {
	l := NewLimiter("test", 0)
	if l.Max() != 1 {
		t.Errorf("max = %d, want 1 (should floor to 1)", l.Max())
	}
}

func TestLimiterName(t *testing.T) {
	l := NewLimiter("goroutines", 10)
	if l.Name() != "goroutines" {
		t.Errorf("name = %s, want goroutines", l.Name())
	}
}

// MemoryBudget tests

func TestMemoryBudgetReserve(t *testing.T) {
	b := NewMemoryBudget("heap", 1000)

	if !b.Reserve(500) {
		t.Error("500 should fit in 1000 budget")
	}
	if b.Reserved() != 500 {
		t.Errorf("reserved = %d, want 500", b.Reserved())
	}
	if b.Available() != 500 {
		t.Errorf("available = %d, want 500", b.Available())
	}
}

func TestMemoryBudgetExceed(t *testing.T) {
	b := NewMemoryBudget("heap", 1000)

	if !b.Reserve(800) {
		t.Error("800 should fit")
	}
	if b.Reserve(300) {
		t.Error("300 should not fit (800 + 300 > 1000)")
	}
}

func TestMemoryBudgetRelease(t *testing.T) {
	b := NewMemoryBudget("heap", 1000)
	b.Reserve(500)
	b.Release(300)

	if b.Reserved() != 200 {
		t.Errorf("reserved = %d, want 200", b.Reserved())
	}
}

func TestMemoryBudgetExactLimit(t *testing.T) {
	b := NewMemoryBudget("heap", 1000)

	if !b.Reserve(1000) {
		t.Error("exact limit should be allowed")
	}
	if b.Reserve(1) {
		t.Error("1 more byte should not fit")
	}
}

func TestMemoryBudgetZero(t *testing.T) {
	b := NewMemoryBudget("heap", 0)
	if b.Reserve(1) {
		t.Error("nothing should fit in zero budget")
	}
}

// Snapshot tests

func TestTakeSnapshot(t *testing.T) {
	snap := TakeSnapshot()

	if snap.HeapBytes <= 0 {
		t.Errorf("heap bytes = %d, want > 0", snap.HeapBytes)
	}
	if snap.Goroutines <= 0 {
		t.Errorf("goroutines = %d, want > 0", snap.Goroutines)
	}
	if snap.NumCPU <= 0 {
		t.Errorf("num_cpu = %d, want > 0", snap.NumCPU)
	}
}

func TestHeapUsage(t *testing.T) {
	heap := HeapUsage()
	if heap <= 0 {
		t.Errorf("heap = %d, want > 0", heap)
	}
}

func TestGoroutineCount(t *testing.T) {
	count := GoroutineCount()
	if count <= 0 {
		t.Errorf("goroutines = %d, want > 0", count)
	}
}

// Monitor tests

func TestMonitorTrack(t *testing.T) {
	mon := NewMonitor()
	l := NewLimiter("goroutines", 100)
	b := NewMemoryBudget("heap", 1<<30)

	mon.Track(l)
	mon.TrackBudget(b)

	l.Acquire(context.Background())
	b.Reserve(1024)

	status := mon.Status()

	gs, ok := status["goroutines"]
	if !ok {
		t.Fatal("goroutines not in status")
	}
	if gs.Active != 1 {
		t.Errorf("goroutines active = %d, want 1", gs.Active)
	}
	if gs.Max != 100 {
		t.Errorf("goroutines max = %d, want 100", gs.Max)
	}

	hs, ok := status["heap"]
	if !ok {
		t.Fatal("heap not in status")
	}
	if hs.Active != 1024 {
		t.Errorf("heap active = %d, want 1024", hs.Active)
	}
	if hs.Max != 1<<30 {
		t.Errorf("heap max = %d, want %d", hs.Max, 1<<30)
	}

	l.Release()
}

// Stress tests

func TestLimiterConcurrent(t *testing.T) {
	l := NewLimiter("test", 10)
	var wg sync.WaitGroup
	var maxActive atomic.Int64

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l.Acquire(context.Background())
			cur := l.Active()
			for {
				old := maxActive.Load()
				if cur <= old || maxActive.CompareAndSwap(old, cur) {
					break
				}
			}
			time.Sleep(time.Millisecond)
			l.Release()
		}()
	}

	wg.Wait()

	if maxActive.Load() > 10 {
		t.Errorf("max active = %d, exceeded limit of 10", maxActive.Load())
	}
	if l.Active() != 0 {
		t.Errorf("active = %d, want 0 after all released", l.Active())
	}
	if l.Total() != 100 {
		t.Errorf("total = %d, want 100", l.Total())
	}
}

func TestLimiterGoConcurrent(t *testing.T) {
	l := NewLimiter("test", 5)
	var wg sync.WaitGroup
	var completed atomic.Int32

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := l.Go(context.Background(), func() {
				time.Sleep(time.Millisecond)
				completed.Add(1)
			})
			if err != nil {
				t.Errorf("Go: %v", err)
			}
		}()
	}

	wg.Wait()
	// Wait for background goroutines to finish.
	time.Sleep(100 * time.Millisecond)

	if completed.Load() != 50 {
		t.Errorf("completed = %d, want 50", completed.Load())
	}
}

func TestMemoryBudgetConcurrent(t *testing.T) {
	b := NewMemoryBudget("heap", 10000)
	var wg sync.WaitGroup
	var reserved atomic.Int64

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if b.Reserve(100) {
				reserved.Add(100)
				time.Sleep(time.Millisecond)
				b.Release(100)
				reserved.Add(-100)
			}
		}()
	}

	wg.Wait()

	if b.Reserved() != 0 {
		t.Errorf("reserved = %d, want 0 after all released", b.Reserved())
	}
}

func TestMonitorConcurrent(t *testing.T) {
	mon := NewMonitor()
	l1 := NewLimiter("a", 10)
	l2 := NewLimiter("b", 10)
	mon.Track(l1)
	mon.Track(l2)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l1.Acquire(context.Background())
			l2.Acquire(context.Background())
			_ = mon.Status()
			l2.Release()
			l1.Release()
		}()
	}
	wg.Wait()
}
