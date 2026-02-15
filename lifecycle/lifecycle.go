// Package lifecycle provides graceful startup and shutdown for MIST tools.
// It handles OS signals (SIGTERM, SIGINT), drains in-flight work, and runs
// shutdown hooks in reverse registration order (LIFO, like defer).
//
// Every MIST tool should wrap its main logic in lifecycle.Run:
//
//	func main() {
//	    err := lifecycle.Run(func(ctx context.Context) error {
//	        dg := lifecycle.DrainGroup(ctx)
//	        lifecycle.OnShutdown(ctx, func() error {
//	            return server.Close()
//	        })
//	        return server.ListenAndServe()
//	    })
//	    if err != nil { os.Exit(1) }
//	}
//
// On SIGTERM/SIGINT, the context is cancelled, drain groups are awaited
// (with timeout), then shutdown hooks run in reverse order.
package lifecycle

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type contextKey struct{}

// state holds the lifecycle state for a single Run invocation.
type state struct {
	mu       sync.Mutex
	hooks    []func() error
	drains   []*sync.WaitGroup
	drainTTL time.Duration
	shutTTL  time.Duration
}

// Option configures lifecycle behavior.
type Option func(*state)

// WithDrainTimeout sets the maximum time to wait for in-flight work to drain.
// Default: 15 seconds.
func WithDrainTimeout(d time.Duration) Option {
	return func(s *state) { s.drainTTL = d }
}

// WithShutdownTimeout sets the maximum time for shutdown hooks to complete.
// Default: 10 seconds.
func WithShutdownTimeout(d time.Duration) Option {
	return func(s *state) { s.shutTTL = d }
}

// Run executes fn with a context that is cancelled on SIGTERM or SIGINT.
// After fn returns (or the context is cancelled), Run:
//  1. Waits for all drain groups to finish (with timeout)
//  2. Runs shutdown hooks in reverse order (with timeout)
//  3. Returns the first error encountered
//
// Panics in fn are recovered and returned as errors.
func Run(fn func(ctx context.Context) error, opts ...Option) (retErr error) {
	st := &state{
		drainTTL: 15 * time.Second,
		shutTTL:  10 * time.Second,
	}
	for _, o := range opts {
		o(st)
	}

	// Create context that cancels on signal.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	// Attach state to context for OnShutdown/DrainGroup.
	ctx = context.WithValue(ctx, contextKey{}, st)

	// Run main function.
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("panic: %v", r)
			}
		}()
		done <- fn(ctx)
	}()

	// Wait for main to return or signal.
	select {
	case retErr = <-done:
		cancel()
	case sig := <-sigCh:
		cancel()
		// Wait briefly for main to notice cancellation.
		select {
		case fnErr := <-done:
			if retErr == nil {
				retErr = fnErr
			}
		case <-time.After(st.drainTTL):
		}
		_ = sig
	}

	// Phase 1: Drain in-flight work.
	if err := st.drain(); err != nil {
		if retErr == nil {
			retErr = err
		}
	}

	// Phase 2: Run shutdown hooks (reverse order).
	if err := st.shutdown(); err != nil {
		if retErr == nil {
			retErr = err
		}
	}

	return retErr
}

// OnShutdown registers a function to run during shutdown. Hooks run in
// reverse registration order (LIFO). The context must come from Run.
func OnShutdown(ctx context.Context, fn func() error) {
	st := stateFromContext(ctx)
	if st == nil {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	st.hooks = append(st.hooks, fn)
}

// DrainGroup returns a WaitGroup that lifecycle.Run will wait on before
// running shutdown hooks. Use it to track in-flight work:
//
//	dg := lifecycle.DrainGroup(ctx)
//	dg.Add(1)
//	go func() {
//	    defer dg.Done()
//	    processMessage(msg)
//	}()
func DrainGroup(ctx context.Context) *sync.WaitGroup {
	st := stateFromContext(ctx)
	if st == nil {
		return &sync.WaitGroup{}
	}
	wg := &sync.WaitGroup{}
	st.mu.Lock()
	defer st.mu.Unlock()
	st.drains = append(st.drains, wg)
	return wg
}

// drain waits for all drain groups to complete, with timeout.
func (s *state) drain() error {
	s.mu.Lock()
	drains := make([]*sync.WaitGroup, len(s.drains))
	copy(drains, s.drains)
	s.mu.Unlock()

	if len(drains) == 0 {
		return nil
	}

	done := make(chan struct{})
	go func() {
		for _, wg := range drains {
			wg.Wait()
		}
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(s.drainTTL):
		return fmt.Errorf("lifecycle: drain timeout after %v", s.drainTTL)
	}
}

// shutdown runs hooks in reverse order with timeout.
func (s *state) shutdown() error {
	s.mu.Lock()
	hooks := make([]func() error, len(s.hooks))
	copy(hooks, s.hooks)
	s.mu.Unlock()

	if len(hooks) == 0 {
		return nil
	}

	type hookResult struct {
		err error
	}

	done := make(chan hookResult, 1)
	go func() {
		var firstErr error
		// Run in reverse order (LIFO).
		for i := len(hooks) - 1; i >= 0; i-- {
			if err := hooks[i](); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		done <- hookResult{err: firstErr}
	}()

	select {
	case result := <-done:
		return result.err
	case <-time.After(s.shutTTL):
		return fmt.Errorf("lifecycle: shutdown timeout after %v", s.shutTTL)
	}
}

func stateFromContext(ctx context.Context) *state {
	st, _ := ctx.Value(contextKey{}).(*state)
	return st
}
