package server

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewServer(t *testing.T) {
	t.Run("creates server with correct address", func(t *testing.T) {
		addr := "localhost:8080"
		s := New(addr)
		
		if s.Addr != addr {
			t.Errorf("expected address %q, got %q", addr, s.Addr)
		}
		
		if s.mux == nil {
			t.Error("expected mux to be initialized")
		}
		
		if s.srv == nil {
			t.Error("expected srv to be initialized")
		}
		
		if s.srv.Addr != addr {
			t.Errorf("expected srv address %q, got %q", addr, s.srv.Addr)
		}
	})

	t.Run("creates server with zero address", func(t *testing.T) {
		s := New(":0")
		
		if s.Addr != ":0" {
			t.Errorf("expected address :0, got %q", s.Addr)
		}
	})
}

func TestHandle(t *testing.T) {
	t.Run("registers handler and responds correctly", func(t *testing.T) {
		s := New(":0")
		
		called := false
		s.Handle("/test", func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("test response"))
		})

		// Start server in background
		errCh := make(chan error, 1)
		go func() {
			errCh <- s.ListenAndServe()
		}()

		// Give server time to start
		time.Sleep(100 * time.Millisecond)

		// Make request
		resp, err := http.Get("http://" + s.srv.Addr + "/test")
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response: %v", err)
		}

		if string(body) != "test response" {
			t.Errorf("expected body 'test response', got %q", string(body))
		}

		if !called {
			t.Error("handler was not called")
		}

		// Shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.srv.Shutdown(ctx); err != nil {
			t.Errorf("shutdown error: %v", err)
		}
	})

	t.Run("multiple handlers", func(t *testing.T) {
		s := New(":0")
		
		s.Handle("/one", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("one"))
		})
		s.Handle("/two", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("two"))
		})

		errCh := make(chan error, 1)
		go func() {
			errCh <- s.ListenAndServe()
		}()

		time.Sleep(100 * time.Millisecond)

		// Test first handler
		resp1, err := http.Get("http://" + s.srv.Addr + "/one")
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp1.Body.Close()

		body1, _ := io.ReadAll(resp1.Body)
		if string(body1) != "one" {
			t.Errorf("expected 'one', got %q", string(body1))
		}

		// Test second handler
		resp2, err := http.Get("http://" + s.srv.Addr + "/two")
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp2.Body.Close()

		body2, _ := io.ReadAll(resp2.Body)
		if string(body2) != "two" {
			t.Errorf("expected 'two', got %q", string(body2))
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.srv.Shutdown(ctx)
	})
}

func TestMux(t *testing.T) {
	t.Run("returns underlying mux", func(t *testing.T) {
		s := New(":0")
		mux := s.Mux()
		
		if mux == nil {
			t.Error("expected mux to be non-nil")
		}
		
		if mux != s.mux {
			t.Error("expected Mux() to return the same mux instance")
		}
	})

	t.Run("mux can be used directly", func(t *testing.T) {
		s := New(":0")
		
		// Register handler directly via Mux()
		s.Mux().HandleFunc("/direct", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("direct"))
		})

		errCh := make(chan error, 1)
		go func() {
			errCh <- s.ListenAndServe()
		}()

		time.Sleep(100 * time.Millisecond)

		resp, err := http.Get("http://" + s.srv.Addr + "/direct")
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if string(body) != "direct" {
			t.Errorf("expected 'direct', got %q", string(body))
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.srv.Shutdown(ctx)
	})
}

func TestServerTimeouts(t *testing.T) {
	t.Run("server has reasonable timeout values", func(t *testing.T) {
		s := New(":0")
		
		if s.srv.ReadHeaderTimeout != 10*time.Second {
			t.Errorf("expected ReadHeaderTimeout 10s, got %v", s.srv.ReadHeaderTimeout)
		}
		
		if s.srv.IdleTimeout != 120*time.Second {
			t.Errorf("expected IdleTimeout 120s, got %v", s.srv.IdleTimeout)
		}
	})
}

func TestConcurrentRequests(t *testing.T) {
	t.Run("handles multiple concurrent requests", func(t *testing.T) {
		s := New(":0")
		
		var mu sync.Mutex
		count := 0
		
		s.Handle("/concurrent", func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			count++
			mu.Unlock()
			time.Sleep(10 * time.Millisecond)
			w.Write([]byte("ok"))
		})

		errCh := make(chan error, 1)
		go func() {
			errCh <- s.ListenAndServe()
		}()

		time.Sleep(100 * time.Millisecond)

		// Make 10 concurrent requests
		var wg sync.WaitGroup
		numRequests := 10
		wg.Add(numRequests)

		for i := 0; i < numRequests; i++ {
			go func() {
				defer wg.Done()
				resp, err := http.Get("http://" + s.srv.Addr + "/concurrent")
				if err != nil {
					t.Errorf("request failed: %v", err)
					return
				}
				defer resp.Body.Close()
				
				if resp.StatusCode != http.StatusOK {
					t.Errorf("expected status 200, got %d", resp.StatusCode)
				}
			}()
		}

		wg.Wait()

		mu.Lock()
		finalCount := count
		mu.Unlock()

		if finalCount != numRequests {
			t.Errorf("expected %d requests handled, got %d", numRequests, finalCount)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.srv.Shutdown(ctx)
	})
}

func TestListenAndServeErrors(t *testing.T) {
	t.Run("returns error on invalid address", func(t *testing.T) {
		s := New("invalid-address")
		
		errCh := make(chan error, 1)
		go func() {
			errCh <- s.ListenAndServe()
		}()

		select {
		case err := <-errCh:
			if err == nil {
				t.Error("expected error for invalid address, got nil")
			}
			if !strings.Contains(err.Error(), "server:") {
				t.Errorf("expected error message to contain 'server:', got %v", err)
			}
		case <-time.After(1 * time.Second):
			t.Error("timeout waiting for error")
		}
	})

	t.Run("returns error on address already in use", func(t *testing.T) {
		s1 := New(":0")
		
		errCh1 := make(chan error, 1)
		go func() {
			errCh1 <- s1.ListenAndServe()
		}()

		time.Sleep(100 * time.Millisecond)

		// Try to start second server on same address
		s2 := New(s1.srv.Addr)
		
		errCh2 := make(chan error, 1)
		go func() {
			errCh2 <- s2.ListenAndServe()
		}()

		select {
		case err := <-errCh2:
			if err == nil {
				t.Error("expected error for address in use, got nil")
			}
		case <-time.After(1 * time.Second):
			t.Error("timeout waiting for error")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s1.srv.Shutdown(ctx)
	})
}

func TestGracefulShutdown(t *testing.T) {
	t.Run("server shuts down cleanly", func(t *testing.T) {
		s := New(":0")
		
		s.Handle("/test", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("ok"))
		})

		errCh := make(chan error, 1)
		go func() {
			errCh <- s.ListenAndServe()
		}()

		time.Sleep(100 * time.Millisecond)

		// Make a request to ensure server is running
		resp, err := http.Get("http://" + s.srv.Addr + "/test")
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		resp.Body.Close()

		// Shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		shutdownErr := s.srv.Shutdown(ctx)
		if shutdownErr != nil {
			t.Errorf("shutdown error: %v", shutdownErr)
		}

		// Verify server stopped
		select {
		case err := <-errCh:
			// Should get http.ErrServerClosed or nil
			if err != nil && err != http.ErrServerClosed {
				t.Errorf("unexpected error after shutdown: %v", err)
			}
		case <-time.After(6 * time.Second):
			t.Error("timeout waiting for server to stop")
		}
	})
}
