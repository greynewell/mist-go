package transport

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/greynewell/mist-go/protocol"
)

func TestFileSendReceive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "messages.jsonl")

	ft, err := NewFile(path)
	if err != nil {
		t.Fatalf("NewFile: %v", err)
	}
	defer ft.Close()

	ctx := context.Background()
	msgs := make([]*protocol.Message, 3)
	for i := range msgs {
		m, err := protocol.New(protocol.SourceSchemaFlux, protocol.TypeDataEntities, protocol.DataEntities{
			Count:  i + 1,
			Format: "jsonl",
		})
		if err != nil {
			t.Fatalf("protocol.New: %v", err)
		}
		msgs[i] = m
		if err := ft.Send(ctx, m); err != nil {
			t.Fatalf("Send[%d]: %v", i, err)
		}
	}

	// Read them back with a new File transport to test independent read.
	ft2, _ := NewFile(path)
	defer ft2.Close()

	for i := range msgs {
		got, err := ft2.Receive(ctx)
		if err != nil {
			t.Fatalf("Receive[%d]: %v", i, err)
		}
		if got.ID != msgs[i].ID {
			t.Errorf("msg[%d].ID = %q, want %q", i, got.ID, msgs[i].ID)
		}
	}
}

func TestFileReceiveNoMoreMessages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.jsonl")

	// Create empty file.
	os.WriteFile(path, []byte{}, 0644)

	ft, _ := NewFile(path)
	defer ft.Close()

	_, err := ft.Receive(context.Background())
	if err == nil {
		t.Error("expected error when no messages")
	}
}

func TestFileClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "close.jsonl")

	ft, _ := NewFile(path)

	// Send one message to open the writer.
	msg, _ := protocol.New("test", protocol.TypeHealthPing, protocol.HealthPing{From: "test"})
	ft.Send(context.Background(), msg)

	// Receive to open the reader.
	ft.Receive(context.Background())

	if err := ft.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
