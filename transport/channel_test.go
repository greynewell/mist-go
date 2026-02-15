package transport

import (
	"context"
	"testing"

	"github.com/greynewell/mist-go/protocol"
)

func TestChannelSendReceive(t *testing.T) {
	ch := NewChannel(16)
	ctx := context.Background()

	msg, err := protocol.New(protocol.SourceMatchSpec, protocol.TypeHealthPing, protocol.HealthPing{From: "test"})
	if err != nil {
		t.Fatalf("protocol.New: %v", err)
	}

	if err := ch.Send(ctx, msg); err != nil {
		t.Fatalf("Send: %v", err)
	}

	got, err := ch.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}

	if got.ID != msg.ID {
		t.Errorf("ID = %q, want %q", got.ID, msg.ID)
	}
}

func TestChannelPair(t *testing.T) {
	a, b := NewChannelPair(16)
	ctx := context.Background()

	// Send from A, receive on B.
	msg1, _ := protocol.New("a", protocol.TypeHealthPing, protocol.HealthPing{From: "a"})
	if err := a.Send(ctx, msg1); err != nil {
		t.Fatalf("a.Send: %v", err)
	}
	got1, err := b.Receive(ctx)
	if err != nil {
		t.Fatalf("b.Receive: %v", err)
	}
	if got1.ID != msg1.ID {
		t.Error("B should receive what A sent")
	}

	// Send from B, receive on A.
	msg2, _ := protocol.New("b", protocol.TypeHealthPong, protocol.HealthPong{From: "b"})
	if err := b.Send(ctx, msg2); err != nil {
		t.Fatalf("b.Send: %v", err)
	}
	got2, err := a.Receive(ctx)
	if err != nil {
		t.Fatalf("a.Receive: %v", err)
	}
	if got2.ID != msg2.ID {
		t.Error("A should receive what B sent")
	}
}

func TestChannelBufferFull(t *testing.T) {
	ch := NewChannel(1)
	ctx := context.Background()

	msg, _ := protocol.New("test", protocol.TypeHealthPing, protocol.HealthPing{From: "test"})

	// First send should succeed.
	if err := ch.Send(ctx, msg); err != nil {
		t.Fatalf("first Send: %v", err)
	}

	// Second send should fail (buffer full).
	if err := ch.Send(ctx, msg); err == nil {
		t.Error("expected buffer full error")
	}
}

func TestChannelSendCancelledContext(t *testing.T) {
	ch := NewChannel(0) // zero buffer means sends always block/fail
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	msg, _ := protocol.New("test", protocol.TypeHealthPing, protocol.HealthPing{From: "test"})
	err := ch.Send(ctx, msg)
	if err == nil {
		t.Error("expected error with cancelled context")
	}
}

func TestChannelReceiveCancelledContext(t *testing.T) {
	ch := NewChannel(16)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ch.Receive(ctx)
	if err == nil {
		t.Error("expected error with cancelled context")
	}
}

func TestChannelClose(t *testing.T) {
	ch := NewChannel(16)
	if err := ch.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Double close should not panic.
	if err := ch.Close(); err != nil {
		t.Fatalf("double Close: %v", err)
	}
}
