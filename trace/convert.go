package trace

import (
	"context"
	"time"

	"github.com/greynewell/mist-go/protocol"
)

// ToProto converts a Span to a protocol.TraceSpan for transport.
func (s *Span) ToProto() protocol.TraceSpan {
	return protocol.TraceSpan{
		TraceID:   s.TraceID,
		SpanID:    s.SpanID,
		ParentID:  s.ParentID,
		Operation: s.Operation,
		StartNS:   s.StartNS,
		EndNS:     s.EndNS,
		Status:    s.Status,
		Attrs:     s.Attrs(),
	}
}

// FromProto creates a Span from a protocol.TraceSpan received over transport.
// The returned span is already ended and should not be modified.
func FromProto(ts protocol.TraceSpan) *Span {
	attrs := ts.Attrs
	if attrs == nil {
		attrs = make(map[string]any)
	}
	return &Span{
		TraceID:   ts.TraceID,
		SpanID:    ts.SpanID,
		ParentID:  ts.ParentID,
		Operation: ts.Operation,
		StartNS:   ts.StartNS,
		EndNS:     ts.EndNS,
		Status:    ts.Status,
		attrs:     attrs,
	}
}

// ContinueFrom starts a child span using the trace context from a received
// protocol.TraceSpan. This is the standard way to propagate traces across
// tool boundaries:
//
//	// Tool B receives a message from Tool A:
//	var span protocol.TraceSpan
//	msg.Decode(&span)
//	ctx, childSpan := trace.ContinueFrom(ctx, span, "process")
//	defer childSpan.End("ok")
func ContinueFrom(ctx context.Context, ts protocol.TraceSpan, operation string) (context.Context, *Span) {
	s := &Span{
		TraceID:   ts.TraceID,
		SpanID:    newID(),
		ParentID:  ts.SpanID,
		Operation: operation,
		StartNS:   time.Now().UnixNano(),
		attrs:     make(map[string]any),
	}
	return context.WithValue(ctx, contextKey{}, s), s
}

// SpanToMessage creates a protocol.Message containing the span as payload.
// Use this to emit spans to TokenTrace.
func SpanToMessage(source string, s *Span) (*protocol.Message, error) {
	return protocol.New(source, protocol.TypeTraceSpan, s.ToProto())
}
