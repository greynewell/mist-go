package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestNewServer(t *testing.T) {
	addr := "localhost:8080"
	s := New(addr)

	if s == nil {
		t.Fatal("New() returned nil")
	}

	if s.Addr != addr {
		t.Errorf("New(%q).Addr = %q, want %q", addr, s.Addr, addr)
	}

	if s.mux == nil {
		t.Error("New() created server with nil mux")
	}

	if s.srv == nil {
		t.Error("New() created server with nil http.Server")
	}

	// Verify timeouts are set
	if s.srv.ReadHeaderTimeout == 0 {
		t.Error("ReadHeaderTimeout not set")
	}

	if s.srv.IdleTimeout == 0 {
		t.Error("IdleTimeout not set")
	}
}

func TestHandle(t *testing.T) {
	s := New("localhost:0") // port 0 = random available port

	called := false
	s.Handle("/test", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "test response")
	})

	// Start server in background
	go func() {
		s.ListenAndServe()
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Get actual listening address
	resp, err := http.Get("http://" + s.srv.Addr + "/test")
	if err != nil {
		t.Fatalf("GET /test failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /test status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "test response" {
		t.Errorf("GET /test body = %q, want %q", body, "test response")
	}

	if !called {
		t.Error("handler was not called")
	}

	// Clean shutdown
	s.srv.Shutdown(context.Background())
}

func TestMux(t *testing.T) {
	s := New("localhost:0")

	mux := s.Mux()
	if mux == nil {
		t.Fatal("Mux() returned nil")
	}

	// Verify we can register handlers directly on the mux
	called := false
	mux.HandleFunc("/direct", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	// Start server
	go func() {
		s.ListenAndServe()
	}()

	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://" + s.srv.Addr + "/direct")
	if err != nil {
		t.Fatalf("GET /direct failed: %v", err)
	}
	defer resp.Body.Close()

	if !called {
		t.Error("handler registered via Mux() was not called")
	}

	s.srv.Shutdown(context.Background())
}

func TestServerTimeouts(t *testing.T) {
	s := New("localhost:0")

	if s.srv.ReadHeaderTimeout <= 0 {
		t.Error("ReadHeaderTimeout should be set to a positive value")
	}

	if s.srv.IdleTimeout <= 0 {
		t.Error("IdleTimeout should be set to a positive value")
	}

	// Verify reasonable timeout values
	if s.srv.ReadHeaderTimeout < 1*time.Second {
		t.Errorf("ReadHeaderTimeout = %v, seems too short", s.srv.ReadHeaderTimeout)
	}

	if s.srv.IdleTimeout < 10*time.Second {
		t.Errorf("IdleTimeout = %v, seems too short", s.srv.IdleTimeout)
	}
}

func TestConcurrentRequests(t *testing.T) {
	s := New("localhost:0")

	var requestCount int
	var mu sync.Mutex

	s.Handle("/count", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		time.Sleep(10 * time.Millisecond) // simulate some work
		w.WriteHeader(http.StatusOK)
	})

	go func() {
		s.ListenAndServe()
	}()

	time.Sleep(100 * time.Millisecond)

	// Make 10 concurrent requests
	var wg sync.WaitGroup
	numRequests := 10

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := http.Get("http://" + s.srv.Addr + "/count")
			if err != nil {
				t.Errorf("concurrent request failed: %v", err)
				return
			}
			resp.Body.Close()
		}()
	}

	wg.Wait()

	mu.Lock()
	finalCount := requestCount
	mu.Unlock()

	if finalCount != numRequests {
		t.Errorf("handled %d requests, want %d", finalCount, numRequests)
	}

	s.srv.Shutdown(context.Background())
}

func TestInvalidAddress(t *testing.T) {
	s := New("invalid:address:format")

	err := s.ListenAndServe()
	if err == nil {
		t.Error("ListenAndServe() with invalid address should return error")
	}
}

func TestGracefulShutdown(t *testing.T) {
	s := New("localhost:0")

	s.Handle("/slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	started := make(chan struct{})
	serverDone := make(chan error)

	go func() {
		close(started)
		err := s.ListenAndServe()
		serverDone <- err
	}()

	<-started
	time.Sleep(100 * time.Millisecond) // wait for server to be listening

	// Start a slow request
	requestDone := make(chan struct{})
	go func() {
		http.Get("http://" + s.srv.Addr + "/slow")
		close(requestDone)
	}()

	time.Sleep(50 * time.Millisecond) // ensure request started

	// Trigger graceful shutdown
	go func() {
		time.Sleep(50 * time.Millisecond)
		syscall.Kill(syscall.Getpid(), syscall.SIGINT)
	}()

	// Wait for request to complete
	select {
	case <-requestDone:
		// Good - request completed
	case <-time.After(2 * time.Second):
		t.Error("request did not complete during graceful shutdown")
	}

	// Wait for server to shut down
	select {
	case err := <-serverDone:
		if err != nil && err != http.ErrServerClosed {
			t.Errorf("server shutdown error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Error("server did not shut down within timeout")
	}
}
