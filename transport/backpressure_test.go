package transport

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/greynewell/mist-go/protocol"
)

func TestBlockingSendWaitsForSpace(t *testing.T) {
	ch := NewChannel(1)

	msg1, _ := protocol.New("test", protocol.TypeHealthPing, protocol.HealthPing{From: "a"})
	msg2, _ := protocol.New("test", protocol.TypeHealthPing, protocol.HealthPing{From: "b"})

	// Fill the buffer.
	ch.Send(context.Background(), msg1)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- BlockingSend(ctx, ch, msg2)
	}()

	// Drain to unblock.
	time.Sleep(50 * time.Millisecond)
	ch.Receive(context.Background())

	if err := <-done; err != nil {
		t.Fatalf("BlockingSend: %v", err)
	}
}

func TestBlockingSendContextTimeout(t *testing.T) {
	ch := NewChannel(1)

	msg, _ := protocol.New("test", protocol.TypeHealthPing, protocol.HealthPing{From: "a"})
	ch.Send(context.Background(), msg) // fill

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := BlockingSend(ctx, ch, msg)
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestBlockingSendImmediateOnSpace(t *testing.T) {
	ch := NewChannel(16)

	msg, _ := protocol.New("test", protocol.TypeHealthPing, protocol.HealthPing{From: "a"})

	start := time.Now()
	err := BlockingSend(context.Background(), ch, msg)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("BlockingSend: %v", err)
	}
	if elapsed > 50*time.Millisecond {
		t.Errorf("should have been immediate, took %v", elapsed)
	}
}

func TestBlockingSendConcurrent(t *testing.T) {
	ch := NewChannel(4)

	var sent atomic.Int64
	var wg sync.WaitGroup
	const total = 100

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Receiver.
	receiveDone := make(chan struct{})
	go func() {
		defer close(receiveDone)
		for i := 0; i < total; i++ {
			if _, err := ch.Receive(ctx); err != nil {
				return
			}
		}
	}()

	// 10 concurrent senders, 10 msgs each.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				msg, _ := protocol.New("test", protocol.TypeHealthPing, protocol.HealthPing{From: "s"})
				if err := BlockingSend(ctx, ch, msg); err == nil {
					sent.Add(1)
				}
			}
		}()
	}

	wg.Wait()
	<-receiveDone

	if sent.Load() != total {
		t.Errorf("sent = %d, want %d", sent.Load(), total)
	}
}
