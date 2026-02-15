package parallel

import (
	"context"
	"sync"
	"time"
)

// RateLimiter enforces a maximum number of operations per time window
// using a token bucket algorithm. Zero external dependencies.
type RateLimiter struct {
	mu       sync.Mutex
	rate     int           // tokens per interval
	interval time.Duration // refill interval
	tokens   int           // current tokens
	max      int           // bucket capacity
	last     time.Time     // last refill
}

// NewRateLimiter creates a limiter that allows rate operations per interval.
// Burst capacity equals rate (one full interval of tokens).
func NewRateLimiter(rate int, interval time.Duration) *RateLimiter {
	if rate < 1 {
		rate = 1
	}
	return &RateLimiter{
		rate:     rate,
		interval: interval,
		tokens:   rate,
		max:      rate,
		last:     time.Now(),
	}
}

// Wait blocks until a token is available or ctx is cancelled.
func (r *RateLimiter) Wait(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if r.take() {
			return nil
		}

		// Sleep for a fraction of the interval then retry.
		wait := r.interval / time.Duration(r.rate)
		if wait < time.Millisecond {
			wait = time.Millisecond
		}

		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// TryTake attempts to consume a token without blocking.
// Returns true if a token was available.
func (r *RateLimiter) TryTake() bool {
	return r.take()
}

func (r *RateLimiter) take() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.refill()
	if r.tokens > 0 {
		r.tokens--
		return true
	}
	return false
}

func (r *RateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(r.last)
	if elapsed < r.interval {
		// Proportional refill for partial intervals.
		add := int(float64(r.rate) * float64(elapsed) / float64(r.interval))
		if add > 0 {
			r.tokens += add
			if r.tokens > r.max {
				r.tokens = r.max
			}
			r.last = now
		}
		return
	}

	// Full intervals elapsed — refill to max.
	r.tokens = r.max
	r.last = now
}
