// Package transport provides the communication layer for MIST tools.
// Each transport implements the same interface, allowing tools to
// communicate over HTTP, files, stdio pipes, or in-process channels
// without changing application code.
//
// Use Dial to create a transport from a URL:
//
//	t, err := transport.Dial("http://localhost:8081")   // HTTP
//	t, err := transport.Dial("file:///tmp/traces.jsonl") // file
//	t, err := transport.Dial("stdio://")                 // stdin/stdout
//	t, err := transport.Dial("chan://")                   // in-process
package transport

import (
	"context"
	"fmt"
	"strings"

	"github.com/greynewell/mist-go/protocol"
)

// Transport is the interface for bidirectional message passing between
// MIST tools. Implementations must be safe for concurrent use.
type Transport interface {
	Sender
	Receiver
	Close() error
}

// Sender can send messages to a remote tool.
type Sender interface {
	Send(ctx context.Context, msg *protocol.Message) error
}

// Receiver can receive messages from a remote tool.
type Receiver interface {
	Receive(ctx context.Context) (*protocol.Message, error)
}

// Dial creates a transport from a URL string. The URL scheme determines
// the transport type:
//
//	http:// or https:// → HTTP transport
//	file://             → JSON lines file transport
//	stdio://            → stdin/stdout pipe transport
//	chan://             → in-process Go channel transport
func Dial(url string) (Transport, error) {
	scheme, addr := splitScheme(url)

	switch scheme {
	case "http", "https":
		return NewHTTP(url), nil
	case "file":
		return NewFile(addr)
	case "stdio":
		return NewStdio(), nil
	case "chan":
		return NewChannel(256), nil
	default:
		return nil, fmt.Errorf("transport: unsupported scheme %q in %q", scheme, url)
	}
}

func splitScheme(url string) (scheme, rest string) {
	i := strings.Index(url, "://")
	if i < 0 {
		return "", url
	}
	return url[:i], url[i+3:]
}
