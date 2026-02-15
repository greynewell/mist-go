// Package trace provides context-based distributed tracing for MIST tools.
// Spans propagate through context.Context and convert to/from the MIST
// Token Trace Protocol (MTTP) for transport across tool boundaries.
//
// Every operation that crosses a tool boundary should start a span:
//
//	ctx, span := trace.Start(ctx, "inference")
//	defer span.End("ok")
//	// ... do work ...
//	span.SetAttr("model", "claude-sonnet-4-5-20250929")
//	span.SetAttr("tokens_out", 500)
package trace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type contextKey struct{}

// Span represents a single unit of work within a trace. Spans form a tree:
// each span has a trace ID (shared across the request), a unique span ID,
// and an optional parent span ID.
type Span struct {
	TraceID   string
	SpanID    string
	ParentID  string
	Operation string
	StartNS   int64
	Status    string // set by End
	EndNS     int64  // set by End

	mu    sync.Mutex
	attrs map[string]any
}

// Start creates a new span and attaches it to the context. If the context
// already has a span, the new span inherits its trace ID and uses the
// parent's span ID as its parent.
func Start(ctx context.Context, operation string) (context.Context, *Span) {
	s := &Span{
		SpanID:    newID(),
		Operation: operation,
		StartNS:   time.Now().UnixNano(),
		attrs:     make(map[string]any),
	}

	if parent := FromContext(ctx); parent != nil {
		s.TraceID = parent.TraceID
		s.ParentID = parent.SpanID
	} else {
		s.TraceID = newID()
	}

	return context.WithValue(ctx, contextKey{}, s), s
}

// ValidID reports whether an ID contains only printable ASCII characters
// and is within a reasonable length. This prevents log injection via
// malicious trace/span IDs.
func ValidID(id string) bool {
	if id == "" || len(id) > 256 {
		return false
	}
	for _, ch := range id {
		if ch < 32 || ch > 126 {
			return false
		}
	}
	return true
}

// StartWithTraceID creates a span with an explicit trace ID. Use this when
// receiving a message from another tool that includes a trace ID.
// Invalid trace IDs are replaced with a new random ID.
func StartWithTraceID(ctx context.Context, traceID, operation string) (context.Context, *Span) {
	if !ValidID(traceID) {
		traceID = newID()
	}

	s := &Span{
		TraceID:   traceID,
		SpanID:    newID(),
		Operation: operation,
		StartNS:   time.Now().UnixNano(),
		attrs:     make(map[string]any),
	}

	if parent := FromContext(ctx); parent != nil {
		s.ParentID = parent.SpanID
	}

	return context.WithValue(ctx, contextKey{}, s), s
}

// End marks the span as complete with the given status ("ok" or "error").
func (s *Span) End(status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = status
	s.EndNS = time.Now().UnixNano()
}

// SetAttr sets a key-value attribute on the span. Common attributes:
//
//	model, provider, tokens_in, tokens_out, cost_usd, error
func (s *Span) SetAttr(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attrs[key] = value
}

// Attrs returns a copy of the span's attributes.
func (s *Span) Attrs() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make(map[string]any, len(s.attrs))
	for k, v := range s.attrs {
		cp[k] = v
	}
	return cp
}

// DurationNS returns the span duration in nanoseconds, or 0 if not ended.
func (s *Span) DurationNS() int64 {
	if s.EndNS == 0 {
		return 0
	}
	return s.EndNS - s.StartNS
}

// DurationMS returns the span duration in milliseconds.
func (s *Span) DurationMS() float64 {
	return float64(s.DurationNS()) / 1e6
}

// FromContext extracts the current span from the context, or nil.
func FromContext(ctx context.Context) *Span {
	s, _ := ctx.Value(contextKey{}).(*Span)
	return s
}

// TraceID extracts the trace ID from the context, or empty string.
func TraceID(ctx context.Context) string {
	if s := FromContext(ctx); s != nil {
		return s.TraceID
	}
	return ""
}

// SpanID extracts the span ID from the context, or empty string.
func SpanID(ctx context.Context) string {
	if s := FromContext(ctx); s != nil {
		return s.SpanID
	}
	return ""
}

// NewID generates a random 128-bit hex ID suitable for trace and span IDs.
func NewID() string {
	return newID()
}

func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("mist: crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
