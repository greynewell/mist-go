package transport

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/greynewell/mist-go/protocol"
)

// Stdio reads messages from stdin and writes to stdout, one JSON line
// per message. This enables Unix-style piping between MIST tools:
//
//	schemaflux build --output stdio | matchspec run --input stdio
type Stdio struct {
	mu      sync.Mutex
	scanner *bufio.Scanner
}

// NewStdio creates a stdio transport.
func NewStdio() *Stdio {
	s := bufio.NewScanner(os.Stdin)
	s.Buffer(make([]byte, 1<<20), 1<<20)
	return &Stdio{scanner: s}
}

// Send writes a JSON-encoded message to stdout.
func (s *Stdio) Send(_ context.Context, msg *protocol.Message) error {
	data, err := msg.Marshal()
	if err != nil {
		return fmt.Errorf("stdio transport: marshal: %w", err)
	}
	data = append(data, '\n')

	s.mu.Lock()
	defer s.mu.Unlock()
	_, err = os.Stdout.Write(data)
	return err
}

// Receive reads the next JSON line from stdin.
func (s *Stdio) Receive(_ context.Context) (*protocol.Message, error) {
	if !s.scanner.Scan() {
		if err := s.scanner.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("stdio transport: stdin closed")
	}
	return protocol.Unmarshal(s.scanner.Bytes())
}

// Close is a no-op for stdio.
func (s *Stdio) Close() error {
	return nil
}
