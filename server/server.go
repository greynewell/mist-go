// Package server provides a minimal HTTP server with graceful shutdown,
// shared across MIST tools that expose APIs.
package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"
)

// Server is a minimal HTTP server that shuts down cleanly on interrupt.
type Server struct {
	Addr string
	mux  *http.ServeMux
	srv  *http.Server
}

// New creates a server bound to the given address.
func New(addr string) *Server {
	mux := http.NewServeMux()
	return &Server{
		Addr: addr,
		mux:  mux,
		srv: &http.Server{
			Addr:              addr,
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       120 * time.Second,
		},
	}
}

// Handle registers a handler for the given pattern.
func (s *Server) Handle(pattern string, handler http.HandlerFunc) {
	s.mux.HandleFunc(pattern, handler)
}

// Mux returns the underlying ServeMux for direct access.
func (s *Server) Mux() *http.ServeMux {
	return s.mux
}

// ListenAndServe starts the server and blocks until interrupted.
func (s *Server) ListenAndServe() error {
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return fmt.Errorf("server: %w", err)
	}
	fmt.Fprintf(os.Stderr, "listening on %s\n", ln.Addr())

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.srv.Serve(ln)
	}()

	select {
	case err := <-errCh:
		if err != http.ErrServerClosed {
			return err
		}
	case <-stop:
		fmt.Fprintln(os.Stderr, "shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.srv.Shutdown(ctx)
	}

	return nil
}
