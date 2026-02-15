package transport

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/greynewell/mist-go/protocol"
	"github.com/greynewell/mist-go/trace"
)

// Middleware wraps a Transport with additional behavior (logging, tracing,
// retry) without changing the underlying transport code.
type Middleware struct {
	inner  Transport
	logger *slog.Logger
	retry  RetryPolicy
}

// RetryPolicy configures retry behavior for middleware. Zero value means
// no retries.
type RetryPolicy struct {
	MaxAttempts int
	InitialWait time.Duration
	MaxWait     time.Duration
	Multiplier  float64
}

// MiddlewareOption configures a Middleware.
type MiddlewareOption func(*Middleware)

// WithLogger adds structured logging to send/receive operations.
func WithLogger(logger *slog.Logger) MiddlewareOption {
	return func(m *Middleware) { m.logger = logger }
}

// WithRetry adds retry with exponential backoff to send operations.
func WithRetry(p RetryPolicy) MiddlewareOption {
	return func(m *Middleware) { m.retry = p }
}

// Wrap creates a middleware-wrapped transport.
func Wrap(t Transport, opts ...MiddlewareOption) *Middleware {
	m := &Middleware{inner: t}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Send sends a message through the wrapped transport with logging,
// tracing, and optional retry.
func (m *Middleware) Send(ctx context.Context, msg *protocol.Message) error {
	start := time.Now()

	// Start a trace span if tracing is active.
	var span *trace.Span
	if trace.FromContext(ctx) != nil {
		ctx, span = trace.Start(ctx, "transport.send")
		span.SetAttr("msg_type", msg.Type)
		span.SetAttr("msg_source", msg.Source)
	}

	var err error
	attempts := 1

	if m.retry.MaxAttempts > 1 {
		err = m.sendWithRetry(ctx, msg, &attempts)
	} else {
		err = m.inner.Send(ctx, msg)
	}

	elapsed := time.Since(start)

	if span != nil {
		span.SetAttr("duration_ms", elapsed.Milliseconds())
		span.SetAttr("attempts", attempts)
		if err != nil {
			span.SetAttr("error", err.Error())
			span.End("error")
		} else {
			span.End("ok")
		}
	}

	if m.logger != nil {
		attrs := []any{
			"msg_type", msg.Type,
			"msg_id", msg.ID,
			"duration_ms", elapsed.Milliseconds(),
			"attempts", attempts,
		}
		if err != nil {
			m.logger.Error("send failed", append(attrs, "error", err)...)
		} else {
			m.logger.Debug("send ok", attrs...)
		}
	}

	return err
}

func (m *Middleware) sendWithRetry(ctx context.Context, msg *protocol.Message, attempts *int) error {
	wait := m.retry.InitialWait
	var lastErr error

	for i := 0; i < m.retry.MaxAttempts; i++ {
		*attempts = i + 1

		if ctx.Err() != nil {
			if lastErr != nil {
				return lastErr
			}
			return ctx.Err()
		}

		lastErr = m.inner.Send(ctx, msg)
		if lastErr == nil {
			return nil
		}

		if i == m.retry.MaxAttempts-1 {
			break
		}

		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return lastErr
		}

		wait = time.Duration(float64(wait) * m.retry.Multiplier)
		if m.retry.MaxWait > 0 && wait > m.retry.MaxWait {
			wait = m.retry.MaxWait
		}
	}

	return fmt.Errorf("send failed after %d attempts: %w", *attempts, lastErr)
}

// Receive reads a message from the wrapped transport with logging and tracing.
func (m *Middleware) Receive(ctx context.Context) (*protocol.Message, error) {
	start := time.Now()

	msg, err := m.inner.Receive(ctx)

	elapsed := time.Since(start)

	if m.logger != nil && err == nil && msg != nil {
		m.logger.Debug("receive",
			"msg_type", msg.Type,
			"msg_id", msg.ID,
			"duration_ms", elapsed.Milliseconds(),
		)
	}

	return msg, err
}

// Close closes the underlying transport.
func (m *Middleware) Close() error {
	return m.inner.Close()
}
