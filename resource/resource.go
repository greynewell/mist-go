// Package resource provides resource management and limits for MIST tools.
// It includes goroutine limiting, memory budget tracking, and file descriptor
// monitoring to prevent resource exhaustion in production.
package resource

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
)

// Limiter controls concurrent resource usage. It implements a semaphore
// pattern with context support for goroutine limiting, connection pooling,
// and any bounded-concurrency scenario.
type Limiter struct {
	sem  chan struct{}
	name string
	max  int

	active atomic.Int64
	total  atomic.Int64
}

// NewLimiter creates a resource limiter with the given concurrency bound.
func NewLimiter(name string, max int) *Limiter {
	if max < 1 {
		max = 1
	}
	return &Limiter{
		sem:  make(chan struct{}, max),
		name: name,
		max:  max,
	}
}

// Acquire claims one slot from the limiter. It blocks until a slot is
// available or the context is cancelled.
func (l *Limiter) Acquire(ctx context.Context) error {
	select {
	case l.sem <- struct{}{}:
		l.active.Add(1)
		l.total.Add(1)
		return nil
	case <-ctx.Done():
		return fmt.Errorf("resource %s: acquire: %w", l.name, ctx.Err())
	}
}

// TryAcquire attempts to claim a slot without blocking. Returns false
// if no slot is available.
func (l *Limiter) TryAcquire() bool {
	select {
	case l.sem <- struct{}{}:
		l.active.Add(1)
		l.total.Add(1)
		return true
	default:
		return false
	}
}

// Release returns one slot to the limiter. Must be called after Acquire
// or a successful TryAcquire.
func (l *Limiter) Release() {
	<-l.sem
	l.active.Add(-1)
}

// Active returns the number of currently held slots.
func (l *Limiter) Active() int64 {
	return l.active.Load()
}

// Total returns the total number of successful acquisitions.
func (l *Limiter) Total() int64 {
	return l.total.Load()
}

// Max returns the concurrency limit.
func (l *Limiter) Max() int {
	return l.max
}

// Name returns the limiter's name.
func (l *Limiter) Name() string {
	return l.name
}

// Go runs fn in a new goroutine, acquiring a slot first. If the context
// is cancelled before a slot is available, fn is not run and the error
// is returned. The slot is released when fn returns.
func (l *Limiter) Go(ctx context.Context, fn func()) error {
	if err := l.Acquire(ctx); err != nil {
		return err
	}
	go func() {
		defer l.Release()
		fn()
	}()
	return nil
}

// MemoryBudget tracks memory usage against a configured limit.
// It uses runtime.ReadMemStats for actual usage and provides
// a reservation system for pre-allocating memory budgets.
type MemoryBudget struct {
	limit    int64
	reserved atomic.Int64
	name     string
}

// NewMemoryBudget creates a memory budget with the given limit in bytes.
func NewMemoryBudget(name string, limitBytes int64) *MemoryBudget {
	return &MemoryBudget{
		limit: limitBytes,
		name:  name,
	}
}

// Reserve attempts to reserve bytes from the budget. Returns false if
// the reservation would exceed the limit.
func (m *MemoryBudget) Reserve(bytes int64) bool {
	for {
		cur := m.reserved.Load()
		if cur+bytes > m.limit {
			return false
		}
		if m.reserved.CompareAndSwap(cur, cur+bytes) {
			return true
		}
	}
}

// Release returns reserved bytes to the budget.
func (m *MemoryBudget) Release(bytes int64) {
	m.reserved.Add(-bytes)
}

// Reserved returns the currently reserved bytes.
func (m *MemoryBudget) Reserved() int64 {
	return m.reserved.Load()
}

// Limit returns the budget limit in bytes.
func (m *MemoryBudget) Limit() int64 {
	return m.limit
}

// Available returns the remaining bytes before the limit.
func (m *MemoryBudget) Available() int64 {
	avail := m.limit - m.reserved.Load()
	if avail < 0 {
		return 0
	}
	return avail
}

// HeapUsage returns the current heap allocation in bytes from the runtime.
func HeapUsage() int64 {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return int64(ms.HeapAlloc)
}

// GoroutineCount returns the current number of goroutines.
func GoroutineCount() int {
	return runtime.NumGoroutine()
}

// Snapshot captures current resource usage.
type Snapshot struct {
	HeapBytes  int64 `json:"heap_bytes"`
	Goroutines int   `json:"goroutines"`
	NumCPU     int   `json:"num_cpu"`
}

// TakeSnapshot captures the current resource state.
func TakeSnapshot() Snapshot {
	return Snapshot{
		HeapBytes:  HeapUsage(),
		Goroutines: GoroutineCount(),
		NumCPU:     runtime.NumCPU(),
	}
}

// Monitor tracks multiple limiters and budgets, providing a unified
// view of resource usage.
type Monitor struct {
	mu       sync.RWMutex
	limiters []*Limiter
	budgets  []*MemoryBudget
}

// NewMonitor creates a resource monitor.
func NewMonitor() *Monitor {
	return &Monitor{}
}

// Track adds a limiter to the monitor.
func (m *Monitor) Track(l *Limiter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.limiters = append(m.limiters, l)
}

// TrackBudget adds a memory budget to the monitor.
func (m *Monitor) TrackBudget(b *MemoryBudget) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.budgets = append(m.budgets, b)
}

// Status returns a map of resource names to their current usage.
func (m *Monitor) Status() map[string]ResourceStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := make(map[string]ResourceStatus, len(m.limiters)+len(m.budgets))
	for _, l := range m.limiters {
		status[l.Name()] = ResourceStatus{
			Active: l.Active(),
			Max:    int64(l.Max()),
			Total:  l.Total(),
		}
	}
	for _, b := range m.budgets {
		status[b.name] = ResourceStatus{
			Active: b.Reserved(),
			Max:    b.Limit(),
		}
	}
	return status
}

// ResourceStatus describes the current state of a tracked resource.
type ResourceStatus struct {
	Active int64 `json:"active"`
	Max    int64 `json:"max"`
	Total  int64 `json:"total,omitempty"`
}
