package transport

import (
	"context"
	"fmt"
	"sync"

	"github.com/greynewell/mist-go/protocol"
)

// Channel is an in-process transport backed by a Go channel. Use this
// when embedding multiple MIST tools in the same binary or for testing.
//
// For bidirectional communication between two tools, create a pair:
//
//	a, b := NewChannelPair(256)
//	// tool A sends on 'a', tool B receives on 'b' and vice versa
type Channel struct {
	send chan *protocol.Message
	recv chan *protocol.Message
	once sync.Once
}

// NewChannel creates a unidirectional channel transport. Messages sent
// appear on the same transport's Receive.
func NewChannel(bufSize int) *Channel {
	ch := make(chan *protocol.Message, bufSize)
	return &Channel{send: ch, recv: ch}
}

// NewChannelPair creates two linked transports. Sending on one delivers
// to the other's Receive, enabling bidirectional in-process communication.
func NewChannelPair(bufSize int) (*Channel, *Channel) {
	aToB := make(chan *protocol.Message, bufSize)
	bToA := make(chan *protocol.Message, bufSize)
	a := &Channel{send: aToB, recv: bToA}
	b := &Channel{send: bToA, recv: aToB}
	return a, b
}

// Send puts a message on the channel.
func (c *Channel) Send(ctx context.Context, msg *protocol.Message) error {
	select {
	case c.send <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("channel transport: buffer full")
	}
}

// Receive reads the next message from the channel.
func (c *Channel) Receive(ctx context.Context) (*protocol.Message, error) {
	select {
	case msg := <-c.recv:
		return msg, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Close closes the send channel.
func (c *Channel) Close() error {
	c.once.Do(func() { close(c.send) })
	return nil
}
