package transport

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/greynewell/mist-go/protocol"
)

// DialFunc creates a new transport connection. It is called by Resilient
// when the initial connection is needed or when reconnection is required.
type DialFunc func() (Transport, error)

// ResilientConfig controls reconnection behavior.
type ResilientConfig struct {
	// ReconnectWait is the initial wait before reconnecting (default 100ms).
	ReconnectWait time.Duration

	// MaxReconnectWait caps the exponential backoff (default 30s).
	MaxReconnectWait time.Duration

	// OnStateChange is called when the connection state changes.
	// States: "connecting", "connected", "disconnected", "closed".
	OnStateChange func(state string)
}

// Resilient wraps a Transport with automatic reconnection. When a Send
// or Receive fails, the transport is replaced by dialing a new connection.
// This provides connection-level resilience on top of the retry middleware.
type Resilient struct {
	dial DialFunc
	cfg  ResilientConfig

	mu     sync.Mutex
	conn   Transport
	closed bool
}

// NewResilient creates a resilient transport that automatically reconnects
// using the given dial function when the underlying connection fails.
func NewResilient(dial DialFunc, cfg ResilientConfig) *Resilient {
	if cfg.ReconnectWait == 0 {
		cfg.ReconnectWait = 100 * time.Millisecond
	}
	if cfg.MaxReconnectWait == 0 {
		cfg.MaxReconnectWait = 30 * time.Second
	}
	return &Resilient{
		dial: dial,
		cfg:  cfg,
	}
}

// Send sends a message, reconnecting if the underlying transport fails.
func (r *Resilient) Send(ctx context.Context, msg *protocol.Message) error {
	conn, err := r.getOrDial(ctx)
	if err != nil {
		return err
	}

	err = conn.Send(ctx, msg)
	if err == nil {
		return nil
	}

	// Connection failed — reconnect and retry once.
	r.disconnect(conn)
	conn, err = r.reconnect(ctx)
	if err != nil {
		return fmt.Errorf("resilient transport: reconnect failed: %w", err)
	}
	return conn.Send(ctx, msg)
}

// Receive receives a message, reconnecting if the underlying transport fails.
func (r *Resilient) Receive(ctx context.Context) (*protocol.Message, error) {
	for {
		conn, err := r.getOrDial(ctx)
		if err != nil {
			return nil, err
		}

		msg, err := conn.Receive(ctx)
		if err == nil && msg != nil {
			return msg, nil
		}

		// Context cancelled — don't reconnect.
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// Connection failed — reconnect.
		r.disconnect(conn)
		if _, err := r.reconnect(ctx); err != nil {
			return nil, fmt.Errorf("resilient transport: reconnect failed: %w", err)
		}
	}
}

// Close closes the resilient transport and the underlying connection.
func (r *Resilient) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.closed = true
	r.setState("closed")

	if r.conn != nil {
		err := r.conn.Close()
		r.conn = nil
		return err
	}
	return nil
}

// getOrDial returns the current connection, dialing if none exists.
func (r *Resilient) getOrDial(ctx context.Context) (Transport, error) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil, fmt.Errorf("resilient transport: closed")
	}
	if r.conn != nil {
		conn := r.conn
		r.mu.Unlock()
		return conn, nil
	}
	r.mu.Unlock()

	return r.reconnect(ctx)
}

// disconnect closes and removes the current connection if it matches.
func (r *Resilient) disconnect(failed Transport) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.conn == failed {
		r.conn.Close()
		r.conn = nil
		r.setState("disconnected")
	}
}

// reconnect dials a new connection with exponential backoff.
func (r *Resilient) reconnect(ctx context.Context) (Transport, error) {
	wait := r.cfg.ReconnectWait

	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		r.mu.Lock()
		if r.closed {
			r.mu.Unlock()
			return nil, fmt.Errorf("resilient transport: closed")
		}
		// Another goroutine may have reconnected while we waited.
		if r.conn != nil {
			conn := r.conn
			r.mu.Unlock()
			return conn, nil
		}
		r.mu.Unlock()

		r.setState("connecting")

		conn, err := r.dial()
		if err == nil {
			r.mu.Lock()
			// Double-check: another goroutine may have won the race.
			if r.conn != nil {
				conn.Close()
				conn = r.conn
			} else {
				r.conn = conn
			}
			r.mu.Unlock()
			r.setState("connected")
			return conn, nil
		}

		// Backoff.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}

		wait *= 2
		if wait > r.cfg.MaxReconnectWait {
			wait = r.cfg.MaxReconnectWait
		}
	}
}

func (r *Resilient) setState(state string) {
	if r.cfg.OnStateChange != nil {
		r.cfg.OnStateChange(state)
	}
}
