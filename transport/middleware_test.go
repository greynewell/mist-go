package transport

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/greynewell/mist-go/protocol"
	"github.com/greynewell/mist-go/trace"
)

func TestMiddlewareSendReceive(t *testing.T) {
	ch := NewChannel(16)
	m := Wrap(ch)

	ctx := context.Background()
	msg, _ := protocol.New("test", protocol.TypeHealthPing, protocol.HealthPing{From: "test"})

	if err := m.Send(ctx, msg); err != nil {
		t.Fatalf("Send: %v", err)
	}

	got, err := m.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if got.ID != msg.ID {
		t.Error("ID mismatch")
	}
}

func TestMiddlewareWithLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	ch := NewChannel(16)
	m := Wrap(ch, WithLogger(logger))

	ctx := context.Background()
	msg, _ := protocol.New("test", protocol.TypeHealthPing, protocol.HealthPing{From: "test"})

	m.Send(ctx, msg)
	m.Receive(ctx)

	output := buf.String()
	if !strings.Contains(output, "send ok") {
		t.Errorf("expected 'send ok' in log: %s", output)
	}
	if !strings.Contains(output, "receive") {
		t.Errorf("expected 'receive' in log: %s", output)
	}
	if !strings.Contains(output, "health.ping") {
		t.Errorf("expected msg_type in log: %s", output)
	}
}

func TestMiddlewareWithTracing(t *testing.T) {
	ch := NewChannel(16)
	m := Wrap(ch)

	ctx, span := trace.Start(context.Background(), "test-op")
	msg, _ := protocol.New("test", protocol.TypeHealthPing, protocol.HealthPing{From: "test"})

	m.Send(ctx, msg)

	// Span should exist in context but middleware creates its own child.
	_ = span
}

func TestMiddlewareWithRetry(t *testing.T) {
	var attempts int
	failTransport := &failingSender{
		failUntil: 2,
		attempts:  &attempts,
		inner:     NewChannel(16),
	}

	m := Wrap(failTransport, WithRetry(RetryPolicy{
		MaxAttempts: 3,
		InitialWait: time.Millisecond,
		MaxWait:     10 * time.Millisecond,
		Multiplier:  2.0,
	}))

	msg, _ := protocol.New("test", protocol.TypeHealthPing, protocol.HealthPing{From: "test"})
	err := m.Send(context.Background(), msg)

	if err != nil {
		t.Fatalf("Send should succeed after retries: %v", err)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestMiddlewareRetryExhausted(t *testing.T) {
	var attempts int
	failTransport := &failingSender{
		failUntil: 10, // always fail
		attempts:  &attempts,
		inner:     NewChannel(16),
	}

	m := Wrap(failTransport, WithRetry(RetryPolicy{
		MaxAttempts: 3,
		InitialWait: time.Millisecond,
		MaxWait:     10 * time.Millisecond,
		Multiplier:  2.0,
	}))

	msg, _ := protocol.New("test", protocol.TypeHealthPing, protocol.HealthPing{From: "test"})
	err := m.Send(context.Background(), msg)

	if err == nil {
		t.Fatal("expected error after exhausted retries")
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestMiddlewareRetryWithCancelledContext(t *testing.T) {
	var attempts int
	failTransport := &failingSender{
		failUntil: 10,
		attempts:  &attempts,
		inner:     NewChannel(16),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	m := Wrap(failTransport, WithRetry(RetryPolicy{
		MaxAttempts: 5,
		InitialWait: time.Second,
		Multiplier:  2.0,
	}))

	msg, _ := protocol.New("test", protocol.TypeHealthPing, protocol.HealthPing{From: "test"})
	err := m.Send(ctx, msg)

	if err == nil {
		t.Error("expected error with cancelled context")
	}
}

func TestMiddlewareClose(t *testing.T) {
	ch := NewChannel(16)
	m := Wrap(ch)
	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestMiddlewareLoggerOnError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	failTransport := &failingSender{
		failUntil: 10,
		attempts:  new(int),
		inner:     NewChannel(16),
	}

	m := Wrap(failTransport, WithLogger(logger))

	msg, _ := protocol.New("test", protocol.TypeHealthPing, protocol.HealthPing{From: "test"})
	m.Send(context.Background(), msg)

	if !strings.Contains(buf.String(), "send failed") {
		t.Errorf("expected 'send failed' in log: %s", buf.String())
	}
}

// failingSender is a test transport that fails the first N sends.
type failingSender struct {
	failUntil int
	attempts  *int
	inner     Transport
}

func (f *failingSender) Send(ctx context.Context, msg *protocol.Message) error {
	*f.attempts++
	if *f.attempts <= f.failUntil {
		return fmt.Errorf("transient error (attempt %d)", *f.attempts)
	}
	return f.inner.Send(ctx, msg)
}

func (f *failingSender) Receive(ctx context.Context) (*protocol.Message, error) {
	return f.inner.Receive(ctx)
}

func (f *failingSender) Close() error {
	return f.inner.Close()
}
