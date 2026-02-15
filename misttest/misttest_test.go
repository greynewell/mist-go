package misttest

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/greynewell/mist-go/protocol"
)

func testMsg(t *testing.T) *protocol.Message {
	t.Helper()
	msg, err := protocol.New("test", protocol.TypeHealthPing, protocol.HealthPing{From: "test"})
	if err != nil {
		t.Fatal(err)
	}
	return msg
}

// MockTransport tests

func TestMockSendRecords(t *testing.T) {
	m := NewMock()
	msg := testMsg(t)

	if err := m.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send: %v", err)
	}

	sent := m.Sent()
	if len(sent) != 1 {
		t.Fatalf("len(sent) = %d, want 1", len(sent))
	}
	if sent[0].ID != msg.ID {
		t.Errorf("sent ID = %s, want %s", sent[0].ID, msg.ID)
	}
}

func TestMockReceiveResponses(t *testing.T) {
	msg1 := testMsg(t)
	msg2 := testMsg(t)
	m := NewMock(msg1, msg2)

	got1, err := m.Receive(context.Background())
	if err != nil {
		t.Fatalf("Receive 1: %v", err)
	}
	if got1.ID != msg1.ID {
		t.Errorf("got1.ID = %s, want %s", got1.ID, msg1.ID)
	}

	got2, err := m.Receive(context.Background())
	if err != nil {
		t.Fatalf("Receive 2: %v", err)
	}
	if got2.ID != msg2.ID {
		t.Errorf("got2.ID = %s, want %s", got2.ID, msg2.ID)
	}
}

func TestMockReceiveBlocksWhenEmpty(t *testing.T) {
	m := NewMock()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := m.Receive(ctx)
	if err == nil {
		t.Error("expected error from context deadline")
	}
}

func TestMockSendError(t *testing.T) {
	m := NewMock()
	m.SetSendError(fmt.Errorf("connection refused"))

	err := m.Send(context.Background(), testMsg(t))
	if err == nil {
		t.Error("expected send error")
	}
}

func TestMockAddResponse(t *testing.T) {
	m := NewMock()
	msg := testMsg(t)
	m.AddResponse(msg)

	got, err := m.Receive(context.Background())
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if got.ID != msg.ID {
		t.Errorf("got.ID = %s, want %s", got.ID, msg.ID)
	}
}

func TestMockReset(t *testing.T) {
	m := NewMock()
	m.Send(context.Background(), testMsg(t))
	m.SetSendError(fmt.Errorf("fail"))

	m.Reset()

	if len(m.Sent()) != 0 {
		t.Error("sent should be empty after reset")
	}
	if err := m.Send(context.Background(), testMsg(t)); err != nil {
		t.Errorf("send after reset should succeed: %v", err)
	}
}

func TestMockClosed(t *testing.T) {
	m := NewMock()
	m.Close()

	if err := m.Send(context.Background(), testMsg(t)); err == nil {
		t.Error("send after close should fail")
	}
	if _, err := m.Receive(context.Background()); err == nil {
		t.Error("receive after close should fail")
	}
}

// FaultTransport tests

func TestFaultNoFaults(t *testing.T) {
	inner := NewMock()
	f := NewFault(inner, FaultConfig{ErrorRate: 0})

	msg := testMsg(t)
	if err := f.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if len(inner.Sent()) != 1 {
		t.Error("message should pass through")
	}
	f.Close()
}

func TestFaultAlwaysFails(t *testing.T) {
	inner := NewMock()
	f := NewFault(inner, FaultConfig{ErrorRate: 1.0})

	err := f.Send(context.Background(), testMsg(t))
	if err == nil {
		t.Error("expected fault error")
	}

	if len(inner.Sent()) != 0 {
		t.Error("message should not reach inner transport")
	}
	f.Close()
}

func TestFaultCustomError(t *testing.T) {
	customErr := fmt.Errorf("custom fault")
	f := NewFault(NewMock(), FaultConfig{
		ErrorRate: 1.0,
		Error:     customErr,
	})

	err := f.Send(context.Background(), testMsg(t))
	if err != customErr {
		t.Errorf("got %v, want %v", err, customErr)
	}
	f.Close()
}

func TestFaultReceiveFails(t *testing.T) {
	msg := testMsg(t)
	f := NewFault(NewMock(msg), FaultConfig{ErrorRate: 1.0})

	_, err := f.Receive(context.Background())
	if err == nil {
		t.Error("expected fault error on receive")
	}
	f.Close()
}

func TestFaultDelay(t *testing.T) {
	f := NewFault(NewMock(), FaultConfig{
		Delay: 50 * time.Millisecond,
	})

	start := time.Now()
	f.Send(context.Background(), testMsg(t))
	elapsed := time.Since(start)

	if elapsed < 40*time.Millisecond {
		t.Errorf("delay too short: %v", elapsed)
	}
	f.Close()
}

func TestFaultDelayContextCancel(t *testing.T) {
	f := NewFault(NewMock(), FaultConfig{
		Delay: 5 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	f.Send(ctx, testMsg(t))
	elapsed := time.Since(start)

	if elapsed > 1*time.Second {
		t.Errorf("should have cancelled quickly, took %v", elapsed)
	}
	f.Close()
}

func TestFaultPartialFailure(t *testing.T) {
	f := NewFault(NewMock(), FaultConfig{ErrorRate: 0.5})

	var successes, failures int
	for i := 0; i < 1000; i++ {
		if err := f.Send(context.Background(), testMsg(t)); err != nil {
			failures++
		} else {
			successes++
		}
	}

	// With 50% error rate and 1000 attempts, we should have a reasonable
	// distribution. Allow wide bounds to avoid flaky tests.
	if successes < 200 || failures < 200 {
		t.Errorf("expected ~50/50 split, got %d successes, %d failures", successes, failures)
	}
	f.Close()
}

// RecordTransport tests

func TestRecordSend(t *testing.T) {
	inner := NewMock()
	r := NewRecord(inner)

	msg := testMsg(t)
	if err := r.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send: %v", err)
	}

	sent := r.Sent()
	if len(sent) != 1 {
		t.Fatalf("len(sent) = %d, want 1", len(sent))
	}
	if sent[0].ID != msg.ID {
		t.Errorf("sent ID = %s, want %s", sent[0].ID, msg.ID)
	}

	// Inner should also have received it.
	if len(inner.Sent()) != 1 {
		t.Error("inner should have received the message")
	}
	r.Close()
}

func TestRecordReceive(t *testing.T) {
	msg := testMsg(t)
	inner := NewMock(msg)
	r := NewRecord(inner)

	got, err := r.Receive(context.Background())
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if got.ID != msg.ID {
		t.Errorf("got.ID = %s, want %s", got.ID, msg.ID)
	}

	received := r.Received()
	if len(received) != 1 {
		t.Fatalf("len(received) = %d, want 1", len(received))
	}
	r.Close()
}

func TestRecordReplay(t *testing.T) {
	msg1 := testMsg(t)
	msg2 := testMsg(t)
	inner := NewMock(msg1, msg2)
	r := NewRecord(inner)

	// Receive both messages.
	r.Receive(context.Background())
	r.Receive(context.Background())

	// Create replay transport.
	replay := r.Replay()

	got1, _ := replay.Receive(context.Background())
	got2, _ := replay.Receive(context.Background())

	if got1.ID != msg1.ID {
		t.Errorf("replay msg1 ID = %s, want %s", got1.ID, msg1.ID)
	}
	if got2.ID != msg2.ID {
		t.Errorf("replay msg2 ID = %s, want %s", got2.ID, msg2.ID)
	}
	r.Close()
}

func TestRecordSendErrorNotRecorded(t *testing.T) {
	inner := NewMock()
	inner.SetSendError(fmt.Errorf("fail"))
	r := NewRecord(inner)

	r.Send(context.Background(), testMsg(t))

	if len(r.Sent()) != 0 {
		t.Error("failed sends should not be recorded")
	}
	r.Close()
}

// Stress tests

func TestMockConcurrentSendReceive(t *testing.T) {
	msgs := make([]*protocol.Message, 100)
	for i := range msgs {
		msgs[i] = testMsg(t)
	}
	m := NewMock(msgs...)

	var wg sync.WaitGroup
	var sendCount, recvCount atomic.Int32

	// Concurrent senders.
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := m.Send(context.Background(), testMsg(t)); err == nil {
				sendCount.Add(1)
			}
		}()
	}

	// Concurrent receivers.
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			if _, err := m.Receive(ctx); err == nil {
				recvCount.Add(1)
			}
		}()
	}

	wg.Wait()

	if sendCount.Load() != 50 {
		t.Errorf("sendCount = %d, want 50", sendCount.Load())
	}
	if recvCount.Load() != 100 {
		t.Errorf("recvCount = %d, want 100", recvCount.Load())
	}
}

func TestFaultConcurrent(t *testing.T) {
	f := NewFault(NewMock(), FaultConfig{ErrorRate: 0.3})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f.Send(context.Background(), testMsg(t))
		}()
	}
	wg.Wait()
	f.Close()
}
