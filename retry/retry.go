// Package retry provides exponential backoff with jitter for MIST tools.
// Use it to wrap any operation that may fail transiently.
//
//	err := retry.Do(ctx, retry.DefaultPolicy, func(ctx context.Context) error {
//	    return transport.Send(ctx, msg)
//	})
package retry

import (
	"context"
	"math"
	"math/rand/v2"
	"time"

	misterrors "github.com/greynewell/mist-go/errors"
)

// Policy configures retry behavior.
type Policy struct {
	MaxAttempts int           // total attempts (1 = no retry)
	InitialWait time.Duration // wait before first retry
	MaxWait     time.Duration // cap on backoff duration
	Multiplier  float64       // backoff multiplier (typically 2.0)
	Jitter      float64       // random factor 0.0–1.0 (0 = no jitter)
}

// DefaultPolicy is a reasonable default: 3 attempts, 100ms initial,
// 5s max, 2x backoff, 25% jitter.
var DefaultPolicy = Policy{
	MaxAttempts: 3,
	InitialWait: 100 * time.Millisecond,
	MaxWait:     5 * time.Second,
	Multiplier:  2.0,
	Jitter:      0.25,
}

// AggressivePolicy retries more aggressively: 5 attempts, 50ms initial.
var AggressivePolicy = Policy{
	MaxAttempts: 5,
	InitialWait: 50 * time.Millisecond,
	MaxWait:     10 * time.Second,
	Multiplier:  2.0,
	Jitter:      0.25,
}

// Classifier determines whether an error is retryable.
// If nil, all non-nil errors are retried.
type Classifier func(error) bool

// Do executes fn with retries according to the policy. It returns the
// last error if all attempts fail, or nil on success.
func Do(ctx context.Context, p Policy, fn func(context.Context) error) error {
	return DoWithClassifier(ctx, p, nil, fn)
}

// DoWithClassifier executes fn with retries, only retrying errors for which
// classify returns true. If classify is nil, all errors are retried.
func DoWithClassifier(ctx context.Context, p Policy, classify Classifier, fn func(context.Context) error) error {
	if p.MaxAttempts < 1 {
		p.MaxAttempts = 1
	}

	var lastErr error
	wait := p.InitialWait

	for attempt := 0; attempt < p.MaxAttempts; attempt++ {
		if ctx.Err() != nil {
			if lastErr != nil {
				return lastErr
			}
			return ctx.Err()
		}

		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}

		// Check if we should retry this error.
		if classify != nil && !classify(lastErr) {
			return lastErr
		}

		// Don't sleep after the last attempt.
		if attempt == p.MaxAttempts-1 {
			break
		}

		// Apply jitter.
		jittered := wait
		if p.Jitter > 0 {
			delta := float64(wait) * p.Jitter
			jittered = time.Duration(float64(wait) + (rand.Float64()*2-1)*delta)
		}

		select {
		case <-time.After(jittered):
		case <-ctx.Done():
			return lastErr
		}

		// Exponential backoff.
		wait = time.Duration(float64(wait) * p.Multiplier)
		if wait > p.MaxWait {
			wait = p.MaxWait
		}
	}

	return lastErr
}

// DoAuto executes fn with retries, automatically classifying errors using
// the MIST errors package. Errors with retryable codes (timeout, transport,
// unavailable, rate_limit) are retried; permanent errors (validation, auth,
// not_found, etc.) stop immediately.
func DoAuto(ctx context.Context, p Policy, fn func(context.Context) error) error {
	return DoWithClassifier(ctx, p, mistClassifier, fn)
}

// mistClassifier uses the MIST error package to determine retryability.
func mistClassifier(err error) bool {
	return misterrors.IsRetryable(err)
}

// Attempts returns the expected number of attempts for a policy.
func (p Policy) Attempts() int {
	return p.MaxAttempts
}

// TotalMaxWait returns the worst-case total wait time across all retries.
func (p Policy) TotalMaxWait() time.Duration {
	var total time.Duration
	wait := p.InitialWait
	for i := 0; i < p.MaxAttempts-1; i++ {
		total += wait
		wait = time.Duration(math.Min(float64(wait)*p.Multiplier, float64(p.MaxWait)))
	}
	return total
}
