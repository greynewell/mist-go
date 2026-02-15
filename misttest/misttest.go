// Package misttest provides testing utilities for MIST tool authors.
// It includes mock transports, fault injection, and record/replay
// functionality for integration testing without real network connections.
package misttest

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/greynewell/mist-go/protocol"
)

// MockTransport is a transport that records sent messages and returns
// configurable responses. Use it to test tool logic without network I/O.
type MockTransport struct {
	mu       sync.Mutex
	sent     []*protocol.Message
	recvMsgs []*protocol.Message
	recvIdx  int
	sendErr  error
	closed   bool
}

// NewMock creates a MockTransport with optional pre-loaded responses.
func NewMock(responses ...*protocol.Message) *MockTransport {
	return &MockTransport{
		recvMsgs: responses,
	}
}

// Send records the message and returns the configured error (if any).
func (m *MockTransport) Send(_ context.Context, msg *protocol.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return fmt.Errorf("mock: transport closed")
	}
	m.sent = append(m.sent, msg)
	return m.sendErr
}

// Receive returns the next pre-loaded response, or blocks until context
// is cancelled if no more responses are available.
func (m *MockTransport) Receive(ctx context.Context) (*protocol.Message, error) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, fmt.Errorf("mock: transport closed")
	}
	if m.recvIdx < len(m.recvMsgs) {
		msg := m.recvMsgs[m.recvIdx]
		m.recvIdx++
		m.mu.Unlock()
		return msg, nil
	}
	m.mu.Unlock()

	<-ctx.Done()
	return nil, ctx.Err()
}

// Close marks the transport as closed.
func (m *MockTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

// Sent returns a copy of all sent messages.
func (m *MockTransport) Sent() []*protocol.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]*protocol.Message, len(m.sent))
	copy(cp, m.sent)
	return cp
}

// SetSendError configures the error returned by future Send calls.
func (m *MockTransport) SetSendError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendErr = err
}

// AddResponse adds a message to the receive queue.
func (m *MockTransport) AddResponse(msg *protocol.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recvMsgs = append(m.recvMsgs, msg)
}

// Reset clears all recorded messages and responses.
func (m *MockTransport) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = nil
	m.recvMsgs = nil
	m.recvIdx = 0
	m.sendErr = nil
	m.closed = false
}

// FaultConfig controls fault injection behavior.
type FaultConfig struct {
	// ErrorRate is the probability [0.0, 1.0] of injecting an error.
	ErrorRate float64

	// Error is the error returned when a fault is injected.
	// Defaults to "fault injected" if nil.
	Error error

	// Delay adds latency before each operation.
	Delay time.Duration

	// DelayJitter adds random jitter up to this duration.
	DelayJitter time.Duration
}

// FaultTransport wraps a transport and injects configurable failures.
// Use it to test error handling and resilience in tool code.
type FaultTransport struct {
	inner Transport
	cfg   FaultConfig
	mu    sync.Mutex
	rng   *rand.Rand
}

// Transport is the interface that FaultTransport wraps. This matches
// the transport.Transport interface without importing the package.
type Transport interface {
	Send(ctx context.Context, msg *protocol.Message) error
	Receive(ctx context.Context) (*protocol.Message, error)
	Close() error
}

// NewFault creates a fault-injecting transport wrapper.
func NewFault(inner Transport, cfg FaultConfig) *FaultTransport {
	if cfg.Error == nil {
		cfg.Error = fmt.Errorf("fault injected")
	}
	return &FaultTransport{
		inner: inner,
		cfg:   cfg,
		rng:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Send sends through the inner transport, possibly injecting a fault.
func (f *FaultTransport) Send(ctx context.Context, msg *protocol.Message) error {
	f.applyDelay(ctx)
	if f.shouldFail() {
		return f.cfg.Error
	}
	return f.inner.Send(ctx, msg)
}

// Receive receives from the inner transport, possibly injecting a fault.
func (f *FaultTransport) Receive(ctx context.Context) (*protocol.Message, error) {
	f.applyDelay(ctx)
	if f.shouldFail() {
		return nil, f.cfg.Error
	}
	return f.inner.Receive(ctx)
}

// Close closes the inner transport.
func (f *FaultTransport) Close() error {
	return f.inner.Close()
}

func (f *FaultTransport) shouldFail() bool {
	if f.cfg.ErrorRate <= 0 {
		return false
	}
	f.mu.Lock()
	r := f.rng.Float64()
	f.mu.Unlock()
	return r < f.cfg.ErrorRate
}

func (f *FaultTransport) applyDelay(ctx context.Context) {
	d := f.cfg.Delay
	if f.cfg.DelayJitter > 0 {
		f.mu.Lock()
		d += time.Duration(f.rng.Int63n(int64(f.cfg.DelayJitter)))
		f.mu.Unlock()
	}
	if d > 0 {
		select {
		case <-time.After(d):
		case <-ctx.Done():
		}
	}
}

// RecordTransport records all sent and received messages for later replay.
// It passes all operations through to the inner transport.
type RecordTransport struct {
	inner    Transport
	mu       sync.Mutex
	sent     []*protocol.Message
	received []*protocol.Message
}

// NewRecord creates a recording transport wrapper.
func NewRecord(inner Transport) *RecordTransport {
	return &RecordTransport{inner: inner}
}

// Send sends through the inner transport and records the message.
func (r *RecordTransport) Send(ctx context.Context, msg *protocol.Message) error {
	err := r.inner.Send(ctx, msg)
	if err == nil {
		r.mu.Lock()
		r.sent = append(r.sent, msg)
		r.mu.Unlock()
	}
	return err
}

// Receive receives from the inner transport and records the message.
func (r *RecordTransport) Receive(ctx context.Context) (*protocol.Message, error) {
	msg, err := r.inner.Receive(ctx)
	if err == nil && msg != nil {
		r.mu.Lock()
		r.received = append(r.received, msg)
		r.mu.Unlock()
	}
	return msg, err
}

// Close closes the inner transport.
func (r *RecordTransport) Close() error {
	return r.inner.Close()
}

// Sent returns all successfully sent messages.
func (r *RecordTransport) Sent() []*protocol.Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make([]*protocol.Message, len(r.sent))
	copy(cp, r.sent)
	return cp
}

// Received returns all successfully received messages.
func (r *RecordTransport) Received() []*protocol.Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make([]*protocol.Message, len(r.received))
	copy(cp, r.received)
	return cp
}

// Replay creates a MockTransport pre-loaded with the received messages.
// This allows replaying a recorded conversation for deterministic testing.
func (r *RecordTransport) Replay() *MockTransport {
	r.mu.Lock()
	defer r.mu.Unlock()
	msgs := make([]*protocol.Message, len(r.received))
	copy(msgs, r.received)
	return NewMock(msgs...)
}
