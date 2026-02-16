package metrics

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestStressConcurrentCounterInc(t *testing.T) {
	r := NewRegistry()
	c := r.Counter("concurrent")

	var wg sync.WaitGroup
	const goroutines = 100
	const opsPerGoroutine = 10000

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				c.Inc()
			}
		}()
	}

	wg.Wait()
	if c.Value() != goroutines*opsPerGoroutine {
		t.Errorf("value = %d, want %d", c.Value(), goroutines*opsPerGoroutine)
	}
}

func TestStressConcurrentGaugeUpdates(t *testing.T) {
	r := NewRegistry()
	g := r.Gauge("concurrent_gauge")

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				g.Set(float64(i*1000 + j))
				g.Inc()
				g.Dec()
				g.Add(1)
				g.Add(-1)
			}
		}(i)
	}
	wg.Wait()
	// Should not have panicked or deadlocked.
}

func TestStressConcurrentHistogramObserve(t *testing.T) {
	r := NewRegistry()
	h := r.Histogram("concurrent_hist", DefaultBuckets)

	var wg sync.WaitGroup
	const goroutines = 100
	const opsPerGoroutine = 1000

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				h.Observe(float64(j))
			}
		}(i)
	}

	wg.Wait()
	snap := h.Snapshot()
	if snap.Count != goroutines*opsPerGoroutine {
		t.Errorf("count = %d, want %d", snap.Count, goroutines*opsPerGoroutine)
	}
}

func TestStressConcurrentRegistration(t *testing.T) {
	r := NewRegistry()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("metric_%d", i%10)
			for j := 0; j < 100; j++ {
				r.Counter(name, "worker", fmt.Sprintf("%d", i)).Inc()
				r.Gauge(name + "_gauge").Set(float64(j))
				r.Histogram(name+"_hist", DefaultBuckets).Observe(float64(j))
			}
		}(i)
	}

	wg.Wait()
	snap := r.Snapshot()
	if len(snap.Counters) == 0 {
		t.Error("expected some counters")
	}
}

func TestStressCounterPerformance(t *testing.T) {
	r := NewRegistry()
	c := r.Counter("perf")

	start := time.Now()
	const n = 1_000_000
	for i := 0; i < n; i++ {
		c.Inc()
	}
	elapsed := time.Since(start)

	nsPerOp := elapsed.Nanoseconds() / n
	// CI runners with -race are slower; allow up to 500ns/op.
	if nsPerOp > 500 {
		t.Errorf("counter inc too slow: %dns/op", nsPerOp)
	}
}

func TestStressHistogramPerformance(t *testing.T) {
	r := NewRegistry()
	h := r.Histogram("perf", DefaultBuckets)

	start := time.Now()
	const n = 1_000_000
	for i := 0; i < n; i++ {
		h.Observe(float64(i % 1000))
	}
	elapsed := time.Since(start)

	nsPerOp := elapsed.Nanoseconds() / n
	// CI runners with -race are slower; allow up to 2000ns/op.
	if nsPerOp > 2000 {
		t.Errorf("histogram observe too slow: %dns/op", nsPerOp)
	}
}

func TestStressSnapshotUnderLoad(t *testing.T) {
	r := NewRegistry()
	c := r.Counter("snap_counter")
	g := r.Gauge("snap_gauge")
	h := r.Histogram("snap_hist", DefaultBuckets)

	var wg sync.WaitGroup
	var snapshots atomic.Int64

	// Writers.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10000; j++ {
				c.Inc()
				g.Set(float64(j))
				h.Observe(float64(j))
			}
		}()
	}

	// Concurrent readers.
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = r.Snapshot()
				snapshots.Add(1)
			}
		}()
	}

	wg.Wait()
	if snapshots.Load() != 500 {
		t.Errorf("snapshots = %d, want 500", snapshots.Load())
	}
}
