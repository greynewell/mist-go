package trace

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// W3C Trace Context header names.
const (
	TraceparentHeader = "Traceparent"
	TracestateHeader  = "Tracestate"
)

// traceparentRe matches the W3C traceparent format:
// version(2)-trace_id(32)-parent_id(16)-flags(2)
var traceparentRe = regexp.MustCompile(`^([0-9a-f]{2})-([0-9a-f]{32})-([0-9a-f]{16})-([0-9a-f]{2})$`)

var zeroTraceID = strings.Repeat("0", 32)
var zeroParentID = strings.Repeat("0", 16)

// InjectHTTP writes W3C traceparent and tracestate headers from the
// current span in the context. If the context has no span, this is a no-op.
//
// The traceparent header encodes the trace ID and span ID in the W3C format:
//
//	traceparent: 00-{trace_id_32hex}-{parent_id_16hex}-01
//
// MIST generates 32-hex span IDs; for W3C compatibility, the last 16 hex
// characters are used as the parent-id.
func InjectHTTP(ctx context.Context, h http.Header) {
	span := FromContext(ctx)
	if span == nil {
		return
	}

	traceID := normalizeTraceID(span.TraceID)
	parentID := normalizeParentID(span.SpanID)

	h.Set(TraceparentHeader, FormatTraceparent(traceID, parentID))
	h.Set(TracestateHeader, fmt.Sprintf("mist=%s", span.SpanID))
}

// ExtractHTTP reads the W3C traceparent header and creates a child span.
// If the header is missing or invalid, a new root span is created.
// If a tracestate header is present, it is preserved as a span attribute.
func ExtractHTTP(ctx context.Context, h http.Header, operation string) (context.Context, *Span) {
	tp := h.Get(TraceparentHeader)
	if tp == "" {
		return Start(ctx, operation)
	}

	traceID, parentID, ok := ParseTraceparent(tp)
	if !ok {
		return Start(ctx, operation)
	}

	s := &Span{
		TraceID:   traceID,
		SpanID:    newID(),
		ParentID:  parentID,
		Operation: operation,
		StartNS:   time.Now().UnixNano(),
		attrs:     make(map[string]any),
	}

	// Preserve tracestate if present.
	if ts := h.Get(TracestateHeader); ts != "" {
		s.SetAttr("tracestate", ts)
	}

	return context.WithValue(ctx, contextKey{}, s), s
}

// ParseTraceparent parses a W3C traceparent header value.
// Returns the trace ID, parent ID, and whether the parse succeeded.
// Returns false for invalid formats, all-zero trace IDs, or all-zero parent IDs.
func ParseTraceparent(header string) (traceID, parentID string, ok bool) {
	matches := traceparentRe.FindStringSubmatch(header)
	if matches == nil {
		return "", "", false
	}

	traceID = matches[2]
	parentID = matches[3]

	// W3C spec: all-zero trace-id and parent-id are invalid.
	if traceID == zeroTraceID || parentID == zeroParentID {
		return "", "", false
	}

	return traceID, parentID, true
}

// FormatTraceparent formats a W3C traceparent header value.
func FormatTraceparent(traceID, parentID string) string {
	return fmt.Sprintf("00-%s-%s-01", traceID, parentID)
}

// normalizeTraceID ensures the trace ID is exactly 32 lowercase hex characters.
func normalizeTraceID(id string) string {
	if len(id) >= 32 {
		return id[len(id)-32:]
	}
	return strings.Repeat("0", 32-len(id)) + id
}

// normalizeParentID ensures the parent ID is exactly 16 lowercase hex characters.
func normalizeParentID(id string) string {
	if len(id) >= 16 {
		return id[len(id)-16:]
	}
	return strings.Repeat("0", 16-len(id)) + id
}
