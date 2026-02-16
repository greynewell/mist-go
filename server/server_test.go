package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestNewServer(t *testing.T) {
	addr := "127.0.0.1:8080"
	s := New(addr)

	if s == nil {
		t.Fatal("New() returned nil")
	}

	if s.Addr != addr {
		t.Errorf("New() Addr = %q, want %q", s.Addr, addr)
	}

	if s.mux == nil {
		t.Error("New() mux is nil")
	}

	if s.srv == nil {
		t.Error("New() srv is nil")
	}

	if s.srv.Handler != s.mux {
		t.Error("New() srv.Handler is not the mux")
	}
}

func TestHandle(t *testing.T) {
	s := New("127.0.0.1:0") // port 0 = automatic port assignment

	called := false
	s.Handle("/test", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Use httptest to test the handler without starting the full server
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	s.mux.ServeHTTP(rec, req)

	if !called {
		t.Error("Handler was not called")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Handler returned status %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if body != "test response" {
		t.Errorf("Handler returned body %q, want %q", body, "test response")
	}
}

func TestMux(t *testing.T) {
	s := New("127.0.0.1:0")

	mux := s.Mux()
	if mux == nil {
		t.Fatal("Mux() returned nil")
	}

	if mux != s.mux {
		t.Error("Mux() did not return the underlying mux")
	}

	// Test that we can register handlers directly on the mux
	called := false
	mux.HandleFunc("/direct", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
	})

	req := httptest.NewRequest(http.MethodGet, "/direct", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if !called {
		t.Error("Direct mux handler was not called")
	}

	if rec.Code != http.StatusCreated {
		t.Errorf("Direct mux handler returned status %d, want %d", rec.Code, http.StatusCreated)
	}
}

func TestServerTimeouts(t *testing.T) {
	s := New("127.0.0.1:0")

	if s.srv.ReadHeaderTimeout != 10*time.Second {
		t.Errorf("ReadHeaderTimeout = %v, want %v", s.srv.ReadHeaderTimeout, 10*time.Second)
	}

	if s.srv.IdleTimeout != 120*time.Second {
		t.Errorf("IdleTimeout = %v, want %v", s.srv.IdleTimeout, 120*time.Second)
	}
}

func TestConcurrentRequests(t *testing.T) {
	s := New("127.0.0.1:0")

	count := 0
	mu := sync.Mutex{}

	s.Handle("/counter", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		count++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})

	// Simulate concurrent requests
	numRequests := 10
	var wg sync.WaitGroup
	wg.Add(numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/counter", nil)
			rec := httptest.NewRecorder()
			s.mux.ServeHTTP(rec, req)
		}()
	}

	wg.Wait()

	if count != numRequests {
		t.Errorf("Handler was called %d times, want %d", count, numRequests)
	}
}

func TestServerIntegration(t *testing.T) {
	// Test actual server start/stop with a real listener
	s := New("127.0.0.1:0")

	s.Handle("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong"))
	})

	// Start server in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.ListenAndServe()
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Make a real HTTP request
	resp, err := http.Get("http://" + s.srv.Addr + "/ping")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Response status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if string(body) != "pong" {
		t.Errorf("Response body = %q, want %q", string(body), "pong")
	}

	// Shutdown server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.srv.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	// Verify server stopped (should get error or nil from ListenAndServe)
	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			t.Errorf("ListenAndServe returned unexpected error: %v", err)
		}
	case <-time.After(6 * time.Second):
		t.Error("Server did not stop within timeout")
	}
}

func TestServerInvalidAddress(t *testing.T) {
	s := New("invalid:address:format")

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err == nil {
			t.Error("ListenAndServe() should return error for invalid address")
		}
	case <-time.After(2 * time.Second):
		t.Error("ListenAndServe() did not return error within timeout")
	}
}
