package transport

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/greynewell/mist-go/protocol"
)

// failTransport simulates a transport that fails after N sends.
type failTransport struct {
	mu       sync.Mutex
	sendErr  error
	recvCh   chan *protocol.Message
	closed   bool
	sendCall int
	failAt   int // fail on this send number (0 = never)
}

func newFailTransport(failAt int) *failTransport {
	return &failTransport{
		recvCh: make(chan *protocol.Message, 16),
		failAt: failAt,
	}
}

func (f *failTransport) Send(_ context.Context, msg *protocol.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return fmt.Errorf("transport closed")
	}
	f.sendCall++
	if f.sendErr != nil {
		return f.sendErr
	}
	if f.failAt > 0 && f.sendCall >= f.failAt {
		return fmt.Errorf("simulated failure")
	}
	return nil
}

func (f *failTransport) Receive(ctx context.Context) (*protocol.Message, error) {
	select {
	case msg := <-f.recvCh:
		return msg, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (f *failTransport) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func (f *failTransport) setSendErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sendErr = err
}

func TestResilientSendSuccess(t *testing.T) {
	inner := newFailTransport(0)
	dialCount := 0
	r := NewResilient(func() (Transport, error) {
		dialCount++
		return inner, nil
	}, ResilientConfig{})

	msg, _ := protocol.New("test", protocol.TypeHealthPing, protocol.HealthPing{From: "test"})

	if err := r.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestResilientReceive(t *testing.T) {
	inner := newFailTransport(0)
	r := NewResilient(func() (Transport, error) {
		return inner, nil
	}, ResilientConfig{})

	msg, _ := protocol.New("test", protocol.TypeHealthPing, protocol.HealthPing{From: "test"})
	inner.recvCh <- msg

	got, err := r.Receive(context.Background())
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if got.ID != msg.ID {
		t.Errorf("got ID %s, want %s", got.ID, msg.ID)
	}
	r.Close()
}

func TestResilientReconnectOnSendFailure(t *testing.T) {
	var dialCount atomic.Int32
	var transports []*failTransport

	r := NewResilient(func() (Transport, error) {
		n := dialCount.Add(1)
		ft := newFailTransport(0)
		if n == 1 {
			// First transport always fails sends.
			ft.setSendErr(fmt.Errorf("connection refused"))
		}
		transports = append(transports, ft)
		return ft, nil
	}, ResilientConfig{
		ReconnectWait:    time.Millisecond,
		MaxReconnectWait: 10 * time.Millisecond,
	})
	defer r.Close()

	msg, _ := protocol.New("test", protocol.TypeHealthPing, protocol.HealthPing{From: "test"})

	// Should succeed after reconnecting to second transport.
	err := r.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("Send after reconnect: %v", err)
	}

	if dialCount.Load() < 2 {
		t.Errorf("expected at least 2 dials, got %d", dialCount.Load())
	}
}

func TestResilientReconnectOnReceiveFailure(t *testing.T) {
	var dialCount atomic.Int32

	r := NewResilient(func() (Transport, error) {
		n := dialCount.Add(1)
		ft := newFailTransport(0)
		if n == 1 {
			// First transport returns an error on receive immediately.
			close(ft.recvCh) // causes receive to return nil msg
		} else {
			// Second transport has a message ready.
			msg, _ := protocol.New("test", protocol.TypeHealthPing, protocol.HealthPing{From: "test"})
			ft.recvCh <- msg
		}
		return ft, nil
	}, ResilientConfig{
		ReconnectWait:    time.Millisecond,
		MaxReconnectWait: 10 * time.Millisecond,
	})
	defer r.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	msg, err := r.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive after reconnect: %v", err)
	}
	if msg == nil {
		t.Fatal("expected message, got nil")
	}
	if dialCount.Load() < 2 {
		t.Errorf("expected at least 2 dials, got %d", dialCount.Load())
	}
}

func TestResilientDialFailure(t *testing.T) {
	callCount := 0
	r := NewResilient(func() (Transport, error) {
		callCount++
		if callCount <= 2 {
			return nil, fmt.Errorf("connection refused")
		}
		return newFailTransport(0), nil
	}, ResilientConfig{
		ReconnectWait:    time.Millisecond,
		MaxReconnectWait: 10 * time.Millisecond,
	})
	defer r.Close()

	msg, _ := protocol.New("test", protocol.TypeHealthPing, protocol.HealthPing{From: "test"})

	err := r.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if callCount < 3 {
		t.Errorf("expected at least 3 dial attempts, got %d", callCount)
	}
}

func TestResilientContextCancellation(t *testing.T) {
	r := NewResilient(func() (Transport, error) {
		return nil, fmt.Errorf("always fail")
	}, ResilientConfig{
		ReconnectWait:    time.Millisecond,
		MaxReconnectWait: 10 * time.Millisecond,
	})
	defer r.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	msg, _ := protocol.New("test", protocol.TypeHealthPing, protocol.HealthPing{From: "test"})

	err := r.Send(ctx, msg)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestResilientStateCallback(t *testing.T) {
	var states []string
	var mu sync.Mutex

	inner := newFailTransport(0)
	var dialCount int

	r := NewResilient(func() (Transport, error) {
		dialCount++
		if dialCount == 1 {
			return inner, nil
		}
		return newFailTransport(0), nil
	}, ResilientConfig{
		ReconnectWait:    time.Millisecond,
		MaxReconnectWait: 10 * time.Millisecond,
		OnStateChange: func(state string) {
			mu.Lock()
			states = append(states, state)
			mu.Unlock()
		},
	})
	defer r.Close()

	msg, _ := protocol.New("test", protocol.TypeHealthPing, protocol.HealthPing{From: "test"})

	// Successful send → connected.
	r.Send(context.Background(), msg)

	// Force failure.
	inner.setSendErr(fmt.Errorf("broken"))
	r.Send(context.Background(), msg)

	mu.Lock()
	defer mu.Unlock()

	hasConnected := false
	for _, s := range states {
		if s == "connected" {
			hasConnected = true
		}
	}
	if !hasConnected {
		t.Errorf("expected 'connected' state, got %v", states)
	}
}

func TestResilientClose(t *testing.T) {
	inner := newFailTransport(0)
	r := NewResilient(func() (Transport, error) {
		return inner, nil
	}, ResilientConfig{})

	msg, _ := protocol.New("test", protocol.TypeHealthPing, protocol.HealthPing{From: "test"})
	r.Send(context.Background(), msg)

	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Sends after close should fail.
	err := r.Send(context.Background(), msg)
	if err == nil {
		t.Error("expected error after close")
	}
}

// Stress tests

func TestResilientConcurrentSends(t *testing.T) {
	r := NewResilient(func() (Transport, error) {
		return newFailTransport(0), nil
	}, ResilientConfig{})
	defer r.Close()

	msg, _ := protocol.New("test", protocol.TypeHealthPing, protocol.HealthPing{From: "test"})

	var wg sync.WaitGroup
	errs := make([]error, 100)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = r.Send(context.Background(), msg)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: %v", i, err)
		}
	}
}

func TestResilientReconnectUnderLoad(t *testing.T) {
	var dialCount atomic.Int32
	r := NewResilient(func() (Transport, error) {
		n := dialCount.Add(1)
		ft := newFailTransport(0)
		if n%3 == 0 {
			// Every 3rd connection fails immediately.
			ft.setSendErr(fmt.Errorf("flaky"))
		}
		return ft, nil
	}, ResilientConfig{
		ReconnectWait:    time.Millisecond,
		MaxReconnectWait: 5 * time.Millisecond,
	})
	defer r.Close()

	msg, _ := protocol.New("test", protocol.TypeHealthPing, protocol.HealthPing{From: "test"})

	var wg sync.WaitGroup
	var success atomic.Int32
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := r.Send(ctx, msg); err == nil {
				success.Add(1)
			}
		}()
	}
	wg.Wait()

	if success.Load() == 0 {
		t.Error("expected at least some successful sends")
	}
}
