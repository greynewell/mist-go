package transport

import (
	"context"
	"time"

	"github.com/greynewell/mist-go/protocol"
)

// BlockingSend attempts to send a message, retrying with backoff if the
// transport is full. Unlike a plain Send which returns an error immediately
// on a full buffer, BlockingSend waits for capacity using the context's
// deadline for timeout control.
//
// This provides backpressure: senders slow down naturally when the receiver
// can't keep up, instead of dropping messages or returning errors.
func BlockingSend(ctx context.Context, t Transport, msg *protocol.Message) error {
	// Try immediately first.
	err := t.Send(ctx, msg)
	if err == nil {
		return nil
	}

	// Retry with exponential backoff up to context deadline.
	wait := time.Millisecond
	maxWait := 100 * time.Millisecond

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}

		if err := t.Send(ctx, msg); err == nil {
			return nil
		}

		wait *= 2
		if wait > maxWait {
			wait = maxWait
		}
	}
}
