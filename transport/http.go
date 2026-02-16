package transport

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/greynewell/mist-go/protocol"
)

// HTTP sends messages via HTTP POST and receives via an embedded server.
type HTTP struct {
	target string // URL to POST messages to
	client *http.Client

	mu    sync.Mutex
	inbox chan *protocol.Message
	srv   *http.Server
}

// NewHTTP creates a transport that POSTs messages to the given URL.
// Call ListenForMessages to start receiving messages on a local port.
func NewHTTP(targetURL string) *HTTP {
	return &HTTP{
		target: targetURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
				},
				MaxIdleConns:       10,
				IdleConnTimeout:    30 * time.Second,
				DisableCompression: false,
				ForceAttemptHTTP2:  true,
			},
		},
		inbox: make(chan *protocol.Message, 256),
	}
}

// Send POSTs a message to the target URL.
func (h *HTTP) Send(ctx context.Context, msg *protocol.Message) error {
	data, err := msg.Marshal()
	if err != nil {
		return fmt.Errorf("http transport: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.target, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("http transport: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("http transport: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("http transport: status %d", resp.StatusCode)
	}
	return nil
}

// Receive blocks until a message is available from the local listener.
func (h *HTTP) Receive(ctx context.Context) (*protocol.Message, error) {
	select {
	case msg := <-h.inbox:
		return msg, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ListenForMessages starts an HTTP server that accepts POSTed messages.
// This is used when a tool needs to receive messages from other tools.
func (h *HTTP) ListenForMessages(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /mist", func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
		if err != nil {
			http.Error(w, "read error", http.StatusBadRequest)
			return
		}

		msg, err := protocol.Unmarshal(data)
		if err != nil {
			http.Error(w, "invalid message", http.StatusBadRequest)
			return
		}

		select {
		case h.inbox <- msg:
			w.WriteHeader(http.StatusAccepted)
		default:
			http.Error(w, "inbox full", http.StatusServiceUnavailable)
		}
	})

	h.mu.Lock()
	h.srv = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1MB
	}
	h.mu.Unlock()

	return h.srv.ListenAndServe()
}

// Close shuts down the HTTP server if running.
func (h *HTTP) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return h.srv.Shutdown(ctx)
	}
	return nil
}
