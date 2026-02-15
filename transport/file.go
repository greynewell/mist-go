package transport

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/greynewell/mist-go/protocol"
)

// File reads and writes messages as JSON lines to a file. This is useful
// for batch pipelines, CI/CD, and offline evaluation workflows where
// tools run sequentially rather than as concurrent services.
type File struct {
	path    string
	mu      sync.Mutex
	writer  *os.File
	scanner *bufio.Scanner
	reader  *os.File
}

// NewFile creates a file transport for the given path. The file is
// opened for appending (send) and reading (receive).
func NewFile(path string) (*File, error) {
	return &File{path: path}, nil
}

// Send appends a JSON-encoded message as a single line to the file.
func (f *File) Send(_ context.Context, msg *protocol.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.writer == nil {
		w, err := os.OpenFile(f.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("file transport: %w", err)
		}
		f.writer = w
	}

	data, err := msg.Marshal()
	if err != nil {
		return fmt.Errorf("file transport: marshal: %w", err)
	}
	data = append(data, '\n')

	_, err = f.writer.Write(data)
	return err
}

// Receive reads the next JSON line from the file. It returns io.EOF
// when no more lines are available.
func (f *File) Receive(_ context.Context) (*protocol.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.scanner == nil {
		r, err := os.Open(f.path)
		if err != nil {
			return nil, fmt.Errorf("file transport: %w", err)
		}
		f.reader = r
		f.scanner = bufio.NewScanner(r)
		f.scanner.Buffer(make([]byte, 1<<20), 1<<20) // 1MB line buffer
	}

	if !f.scanner.Scan() {
		if err := f.scanner.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("file transport: no more messages")
	}

	return protocol.Unmarshal(f.scanner.Bytes())
}

// Close releases file handles.
func (f *File) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	var firstErr error
	if f.writer != nil {
		if err := f.writer.Close(); err != nil {
			firstErr = err
		}
	}
	if f.reader != nil {
		if err := f.reader.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
