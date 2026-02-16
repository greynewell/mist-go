package transport

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/greynewell/mist-go/protocol"
)

// TestStressChannelHighThroughput pushes many messages through a channel pair
// with concurrent senders and a single receiver goroutine. This verifies
// zero data loss when senders retry on backpressure.
func TestStressChannelHighThroughput(t *testing.T) {
	const totalMessages = 50_000
	const senders = 10
	const msgsPerSender = totalMessages / senders

	a, b := NewChannelPair(4096)
	ctx := context.Background()

	var sendWg sync.WaitGroup
	for s := 0; s < senders; s++ {
		sendWg.Add(1)
		go func(senderID int) {
			defer sendWg.Done()
			for i := 0; i < msgsPerSender; i++ {
				msg, _ := protocol.New(
					fmt.Sprintf("sender-%d", senderID),
					protocol.TypeTraceSpan,
					protocol.TraceSpan{
						TraceID:   fmt.Sprintf("t-%d-%d", senderID, i),
						SpanID:    fmt.Sprintf("s-%d-%d", senderID, i),
						Operation: "test",
						StartNS:   int64(i),
						EndNS:     int64(i + 100),
						Status:    "ok",
					},
				)
				for {
					if err := a.Send(ctx, msg); err == nil {
						break
					}
					time.Sleep(time.Microsecond)
				}
			}
		}(s)
	}

	// Receiver counts messages until senders are done and channel is drained.
	recvDone := make(chan int64, 1)
	go func() {
		var count int64
		for count < totalMessages {
			msg, err := b.Receive(ctx)
			if err != nil {
				break
			}
			if msg.Type != protocol.TypeTraceSpan {
				t.Errorf("unexpected type: %s", msg.Type)
			}
			count++
		}
		recvDone <- count
	}()

	sendWg.Wait()
	received := <-recvDone

	t.Logf("channel throughput: sent and received %d messages", received)
	if received != totalMessages {
		t.Errorf("received %d, want %d", received, totalMessages)
	}
}

// TestStressChannelPairBidirectional runs bidirectional traffic on channel pairs.
func TestStressChannelPairBidirectional(t *testing.T) {
	const messagesEachWay = 10_000

	a, b := NewChannelPair(256)
	ctx := context.Background()

	var wg sync.WaitGroup

	// A sends to B, B sends to A simultaneously.
	send := func(ch *Channel, name string, count int) {
		defer wg.Done()
		for i := 0; i < count; i++ {
			msg, _ := protocol.New(name, protocol.TypeHealthPing, protocol.HealthPing{From: name})
			for {
				err := ch.Send(ctx, msg)
				if err == nil {
					break
				}
				// Buffer full, yield and retry.
				time.Sleep(time.Microsecond)
			}
		}
	}

	recv := func(ch *Channel, count int) {
		defer wg.Done()
		for i := 0; i < count; i++ {
			_, err := ch.Receive(ctx)
			if err != nil {
				t.Errorf("receive error at %d: %v", i, err)
				return
			}
		}
	}

	wg.Add(4)
	go send(a, "A", messagesEachWay)
	go send(b, "B", messagesEachWay)
	go recv(a, messagesEachWay) // A receives what B sent
	go recv(b, messagesEachWay) // B receives what A sent

	wg.Wait()
}

// TestStressFileTransportLargeBatch writes and reads back a large batch of messages.
func TestStressFileTransportLargeBatch(t *testing.T) {
	const count = 10_000

	dir := t.TempDir()
	path := filepath.Join(dir, "stress.jsonl")

	ft, err := NewFile(path)
	if err != nil {
		t.Fatalf("NewFile: %v", err)
	}

	ctx := context.Background()

	// Write phase.
	start := time.Now()
	for i := 0; i < count; i++ {
		msg, _ := protocol.New(protocol.SourceTokenTrace, protocol.TypeTraceSpan, protocol.TraceSpan{
			TraceID:   fmt.Sprintf("trace-%d", i),
			SpanID:    fmt.Sprintf("span-%d", i),
			Operation: "inference",
			StartNS:   int64(i * 1000000),
			EndNS:     int64(i*1000000 + 500000),
			Status:    "ok",
			Attrs: map[string]any{
				"model":      "test",
				"tokens_in":  float64(100 + i%100),
				"tokens_out": float64(200 + i%500),
			},
		})
		if err := ft.Send(ctx, msg); err != nil {
			t.Fatalf("Send %d: %v", i, err)
		}
	}
	ft.Close()
	writeTime := time.Since(start)

	// Read phase.
	ft2, _ := NewFile(path)
	defer ft2.Close()

	start = time.Now()
	for i := 0; i < count; i++ {
		msg, err := ft2.Receive(ctx)
		if err != nil {
			t.Fatalf("Receive %d: %v", i, err)
		}

		var span protocol.TraceSpan
		if err := msg.Decode(&span); err != nil {
			t.Fatalf("Decode %d: %v", i, err)
		}
		if span.TraceID != fmt.Sprintf("trace-%d", i) {
			t.Fatalf("msg %d: TraceID = %q", i, span.TraceID)
		}
	}
	readTime := time.Since(start)

	t.Logf("file transport: wrote %d msgs in %v, read in %v", count, writeTime, readTime)
}

// TestStressFileLargeMessages tests file transport with large messages.
func TestStressFileLargeMessages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.jsonl")

	ft, _ := NewFile(path)
	ctx := context.Background()

	sizes := []int{1024, 10 * 1024, 100 * 1024, 500 * 1024}
	var sentMsgs []*protocol.Message

	for _, size := range sizes {
		msg, _ := protocol.New(protocol.SourceInferMux, protocol.TypeInferResponse, protocol.InferResponse{
			Content:      strings.Repeat("x", size),
			TokensOut:    int64(size / 4),
			FinishReason: "stop",
		})
		if err := ft.Send(ctx, msg); err != nil {
			t.Fatalf("Send %dKB: %v", size/1024, err)
		}
		sentMsgs = append(sentMsgs, msg)
	}
	ft.Close()

	ft2, _ := NewFile(path)
	defer ft2.Close()

	for i, size := range sizes {
		got, err := ft2.Receive(ctx)
		if err != nil {
			t.Fatalf("Receive %dKB: %v", size/1024, err)
		}
		if got.ID != sentMsgs[i].ID {
			t.Errorf("msg %d: ID mismatch", i)
		}

		var resp protocol.InferResponse
		if err := got.Decode(&resp); err != nil {
			t.Fatalf("Decode %dKB: %v", size/1024, err)
		}
		if len(resp.Content) != size {
			t.Errorf("msg %d: content length = %d, want %d", i, len(resp.Content), size)
		}
	}
}

// TestStressHTTPTransport tests HTTP transport with concurrent clients
// sending to a local server.
func TestStressHTTPTransport(t *testing.T) {
	const clients = 20
	const msgsPerClient = 100

	// Set up a receiving HTTP transport with a test server.
	inbox := make(chan *protocol.Message, clients*msgsPerClient)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /mist", func(w http.ResponseWriter, r *http.Request) {
		data := make([]byte, r.ContentLength)
		r.Body.Read(data)
		msg, err := protocol.Unmarshal(data)
		if err != nil {
			http.Error(w, "bad msg", http.StatusBadRequest)
			return
		}
		select {
		case inbox <- msg:
			w.WriteHeader(http.StatusAccepted)
		default:
			http.Error(w, "full", http.StatusServiceUnavailable)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Concurrent senders.
	var wg sync.WaitGroup
	var sendErrors atomic.Int64
	start := time.Now()

	for c := 0; c < clients; c++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			h := NewHTTP(srv.URL + "/mist")
			ctx := context.Background()

			for i := 0; i < msgsPerClient; i++ {
				msg, _ := protocol.New(
					fmt.Sprintf("client-%d", clientID),
					protocol.TypeTraceSpan,
					protocol.TraceSpan{
						TraceID:   fmt.Sprintf("t-%d-%d", clientID, i),
						SpanID:    fmt.Sprintf("s-%d-%d", clientID, i),
						Operation: "http-stress",
						Status:    "ok",
					},
				)
				if err := h.Send(ctx, msg); err != nil {
					sendErrors.Add(1)
				}
			}
		}(c)
	}

	wg.Wait()
	elapsed := time.Since(start)
	close(inbox)

	var received int
	for range inbox {
		received++
	}

	total := clients * msgsPerClient
	t.Logf("HTTP: %d clients x %d msgs = %d total, received %d, errors %d, time %v",
		clients, msgsPerClient, total, received, sendErrors.Load(), elapsed)

	if sendErrors.Load() > 0 {
		t.Errorf("%d send errors", sendErrors.Load())
	}
	if received != total {
		t.Errorf("received %d, want %d", received, total)
	}
}

// TestStressHTTPLargePayloads sends large messages over HTTP.
func TestStressHTTPLargePayloads(t *testing.T) {
	inbox := make(chan *protocol.Message, 100)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /mist", func(w http.ResponseWriter, r *http.Request) {
		// Read up to 2MB.
		data := make([]byte, 0, 2<<20)
		buf := make([]byte, 32*1024)
		for {
			n, err := r.Body.Read(buf)
			data = append(data, buf[:n]...)
			if err != nil {
				break
			}
		}
		msg, err := protocol.Unmarshal(data)
		if err != nil {
			http.Error(w, "bad msg", http.StatusBadRequest)
			return
		}
		inbox <- msg
		w.WriteHeader(http.StatusAccepted)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	h := NewHTTP(srv.URL + "/mist")
	ctx := context.Background()

	sizes := []int{1024, 10 * 1024, 100 * 1024, 500 * 1024}
	for _, size := range sizes {
		msg, _ := protocol.New(protocol.SourceInferMux, protocol.TypeInferResponse, protocol.InferResponse{
			Content: strings.Repeat("y", size),
		})
		if err := h.Send(ctx, msg); err != nil {
			t.Fatalf("Send %dKB: %v", size/1024, err)
		}

		got := <-inbox
		var resp protocol.InferResponse
		if err := got.Decode(&resp); err != nil {
			t.Fatalf("Decode %dKB: %v", size/1024, err)
		}
		if len(resp.Content) != size {
			t.Errorf("%dKB: content length = %d", size/1024, len(resp.Content))
		}
	}
}

// TestStressHTTPConcurrentBidirectional tests HTTP with concurrent senders
// and a server that processes messages and checks data integrity.
func TestStressHTTPConcurrentBidirectional(t *testing.T) {
	const goroutines = 10
	const msgsPerGoroutine = 200

	var received sync.Map
	mux := http.NewServeMux()
	mux.HandleFunc("POST /mist", func(w http.ResponseWriter, r *http.Request) {
		data := make([]byte, r.ContentLength)
		r.Body.Read(data)
		msg, err := protocol.Unmarshal(data)
		if err != nil {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		received.Store(msg.ID, msg)
		w.WriteHeader(http.StatusAccepted)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	sent := make(map[string]*protocol.Message)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			h := NewHTTP(srv.URL + "/mist")
			ctx := context.Background()

			for i := 0; i < msgsPerGoroutine; i++ {
				msg, _ := protocol.New(
					fmt.Sprintf("g%d", gid),
					protocol.TypeEvalResult,
					protocol.EvalResult{
						Suite:  "stress",
						Task:   fmt.Sprintf("task-%d-%d", gid, i),
						Passed: true,
						Score:  float64(i) / float64(msgsPerGoroutine),
					},
				)
				mu.Lock()
				sent[msg.ID] = msg
				mu.Unlock()

				if err := h.Send(ctx, msg); err != nil {
					t.Errorf("g%d send %d: %v", gid, i, err)
				}
			}
		}(g)
	}

	wg.Wait()

	// Verify all sent messages were received with correct data.
	missing := 0
	corrupted := 0
	for id, sentMsg := range sent {
		val, ok := received.Load(id)
		if !ok {
			missing++
			continue
		}
		gotMsg := val.(*protocol.Message)
		if gotMsg.Source != sentMsg.Source || gotMsg.Type != sentMsg.Type {
			corrupted++
		}
	}

	total := goroutines * msgsPerGoroutine
	t.Logf("sent %d, missing %d, corrupted %d", total, missing, corrupted)

	if missing > 0 {
		t.Errorf("%d messages missing", missing)
	}
	if corrupted > 0 {
		t.Errorf("%d messages corrupted", corrupted)
	}
}

// TestStressChannelDataIntegrity verifies zero data corruption through channels.
func TestStressChannelDataIntegrity(t *testing.T) {
	const count = 50_000
	a, b := NewChannelPair(1024)
	ctx := context.Background()

	var wg sync.WaitGroup

	// Sender.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < count; i++ {
			msg, _ := protocol.New(protocol.SourceMatchSpec, protocol.TypeEvalResult, protocol.EvalResult{
				Suite:  "integrity",
				Task:   fmt.Sprintf("task-%d", i),
				Score:  float64(i) / float64(count),
				Passed: i%2 == 0,
			})
			for {
				err := a.Send(ctx, msg)
				if err == nil {
					break
				}
				time.Sleep(time.Microsecond)
			}
		}
	}()

	// Receiver with full verification.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < count; i++ {
			msg, err := b.Receive(ctx)
			if err != nil {
				t.Errorf("receive %d: %v", i, err)
				return
			}

			var result protocol.EvalResult
			if err := msg.Decode(&result); err != nil {
				t.Errorf("decode %d: %v", i, err)
				return
			}

			expectedTask := fmt.Sprintf("task-%d", i)
			if result.Task != expectedTask {
				t.Errorf("msg %d: task = %q, want %q", i, result.Task, expectedTask)
				return
			}
			if result.Suite != "integrity" {
				t.Errorf("msg %d: suite = %q", i, result.Suite)
				return
			}
		}
	}()

	wg.Wait()
}

// TestStressDialConcurrent verifies Dial is safe for concurrent use.
func TestStressDialConcurrent(t *testing.T) {
	const goroutines = 50

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tr, err := Dial("chan://")
			if err != nil {
				t.Errorf("Dial: %v", err)
				return
			}
			tr.Close()
		}()
	}
	wg.Wait()
}
