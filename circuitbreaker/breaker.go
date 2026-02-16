// Package circuitbreaker prevents cascading failures in distributed systems.
// When a downstream service fails repeatedly, the breaker opens and rejects
// requests immediately instead of waiting for timeouts.
//
// States:
//
//	Closed  → requests pass through normally; failures are counted
//	Open    → requests are rejected immediately with ErrOpen
//	HalfOpen → a limited number of probe requests are allowed through
//
// Usage:
//
//	cb := circuitbreaker.New(circuitbreaker.Config{
//	    Threshold: 5,              // open after 5 consecutive failures
//	    Timeout:   30 * time.Second, // try again after 30s
//	    HalfOpenMax: 1,            // allow 1 probe request
//	})
//
//	err := cb.Do(ctx, func(ctx context.Context) error {
//	    return transport.Send(ctx, msg)
//	})
package circuitbreaker

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// State represents the circuit breaker state.
type State int32

const (
	Closed   State = 0
	Open     State = 1
	HalfOpen State = 2
)

func (s State) String() string {
	switch s {
	case Closed:
		return "closed"
	case Open:
		return "open"
	case HalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// ErrOpen is returned when the circuit breaker is open and rejecting requests.
var ErrOpen = fmt.Errorf("circuit breaker is open")

// Config configures circuit breaker behavior.
type Config struct {
	// Threshold is the number of consecutive failures before the breaker opens.
	Threshold int

	// Timeout is how long the breaker stays open before transitioning to half-open.
	Timeout time.Duration

	// HalfOpenMax is the maximum number of concurrent probe requests
	// allowed in the half-open state.
	HalfOpenMax int
}

// Breaker is a circuit breaker that tracks failures and controls access.
type Breaker struct {
	cfg Config

	mu               sync.Mutex
	state            State
	failures         int64
	successes        int64
	consecutFail     int
	openedAt         time.Time
	halfOpenInFlight int32
}

// New creates a circuit breaker with the given configuration.
func New(cfg Config) *Breaker {
	if cfg.Threshold < 1 {
		cfg.Threshold = 5
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.HalfOpenMax < 1 {
		cfg.HalfOpenMax = 1
	}

	return &Breaker{cfg: cfg}
}

// State returns the current circuit breaker state.
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.currentState()
}

// currentState returns the state, transitioning open→half-open if timeout has elapsed.
// Must be called with mu held.
func (b *Breaker) currentState() State {
	if b.state == Open && time.Since(b.openedAt) >= b.cfg.Timeout {
		b.state = HalfOpen
		b.halfOpenInFlight = 0
	}
	return b.state
}

// Counts returns the total successes and failures since the breaker was created.
func (b *Breaker) Counts() (successes, failures int64) {
	return atomic.LoadInt64(&b.successes), atomic.LoadInt64(&b.failures)
}

// Do executes fn if the circuit breaker allows it. Returns ErrOpen if the
// breaker is open. Context cancellation errors do not count toward the
// failure threshold.
func (b *Breaker) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	if err := b.beforeCall(); err != nil {
		return err
	}

	err := fn(ctx)
	b.afterCall(err, ctx)
	return err
}

// DoWithFallback executes fn if allowed, or calls fallback if the breaker
// is open. The fallback receives the rejection error.
func (b *Breaker) DoWithFallback(ctx context.Context, fn func(ctx context.Context) error, fallback func(ctx context.Context, err error) error) error {
	if err := b.beforeCall(); err != nil {
		return fallback(ctx, err)
	}

	err := fn(ctx)
	b.afterCall(err, ctx)
	if err != nil {
		return err
	}
	return nil
}

// beforeCall checks if the request is allowed through.
func (b *Breaker) beforeCall() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.currentState() {
	case Closed:
		return nil
	case Open:
		return ErrOpen
	case HalfOpen:
		if int(b.halfOpenInFlight) >= b.cfg.HalfOpenMax {
			return ErrOpen
		}
		b.halfOpenInFlight++
		return nil
	}
	return nil
}

// afterCall records the result and transitions state.
func (b *Breaker) afterCall(err error, ctx context.Context) {
	// Don't count context cancellation as a failure — the downstream
	// didn't actually fail, the caller gave up.
	if err != nil && ctx.Err() != nil {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if err == nil {
		atomic.AddInt64(&b.successes, 1)
		b.onSuccess()
	} else {
		atomic.AddInt64(&b.failures, 1)
		b.onFailure()
	}
}

// onSuccess handles a successful call. Must be called with mu held.
func (b *Breaker) onSuccess() {
	switch b.state {
	case Closed:
		b.consecutFail = 0
	case HalfOpen:
		// Probe succeeded — close the circuit.
		b.state = Closed
		b.consecutFail = 0
		b.halfOpenInFlight = 0
	}
}

// onFailure handles a failed call. Must be called with mu held.
func (b *Breaker) onFailure() {
	switch b.state {
	case Closed:
		b.consecutFail++
		if b.consecutFail >= b.cfg.Threshold {
			b.state = Open
			b.openedAt = time.Now()
		}
	case HalfOpen:
		// Probe failed — reopen.
		b.state = Open
		b.openedAt = time.Now()
		b.halfOpenInFlight = 0
	}
}
