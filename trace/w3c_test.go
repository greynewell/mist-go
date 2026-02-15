package trace

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestInjectHTTP(t *testing.T) {
	ctx, span := Start(context.Background(), "test-op")
	_ = span

	h := make(http.Header)
	InjectHTTP(ctx, h)

	tp := h.Get(TraceparentHeader)
	if tp == "" {
		t.Fatal("traceparent header not set")
	}

	parts := strings.Split(tp, "-")
	if len(parts) != 4 {
		t.Fatalf("traceparent has %d parts, want 4: %s", len(parts), tp)
	}

	if parts[0] != "00" {
		t.Errorf("version = %s, want 00", parts[0])
	}
	if len(parts[1]) != 32 {
		t.Errorf("trace-id length = %d, want 32: %s", len(parts[1]), parts[1])
	}
	if len(parts[2]) != 16 {
		t.Errorf("parent-id length = %d, want 16: %s", len(parts[2]), parts[2])
	}
	if parts[3] != "01" {
		t.Errorf("flags = %s, want 01", parts[3])
	}
}

func TestInjectHTTPNoSpan(t *testing.T) {
	h := make(http.Header)
	InjectHTTP(context.Background(), h)

	if tp := h.Get(TraceparentHeader); tp != "" {
		t.Errorf("expected no traceparent, got %s", tp)
	}
}

func TestExtractHTTP(t *testing.T) {
	h := make(http.Header)
	h.Set(TraceparentHeader, "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")

	ctx, span := ExtractHTTP(context.Background(), h, "handle-request")

	if span.TraceID != "0af7651916cd43dd8448eb211c80319c" {
		t.Errorf("trace ID = %s, want 0af7651916cd43dd8448eb211c80319c", span.TraceID)
	}
	if span.ParentID != "b7ad6b7169203331" {
		t.Errorf("parent ID = %s, want b7ad6b7169203331", span.ParentID)
	}
	if span.Operation != "handle-request" {
		t.Errorf("operation = %s, want handle-request", span.Operation)
	}
	if span.SpanID == "" {
		t.Error("span ID should be generated")
	}

	// Verify it's on context.
	got := FromContext(ctx)
	if got != span {
		t.Error("span should be on context")
	}
}

func TestExtractHTTPNoHeader(t *testing.T) {
	h := make(http.Header)

	_, span := ExtractHTTP(context.Background(), h, "handle-request")

	// Should create a new root span.
	if span.TraceID == "" {
		t.Error("should create new trace ID")
	}
	if span.ParentID != "" {
		t.Errorf("root span should have no parent, got %s", span.ParentID)
	}
}

func TestExtractHTTPInvalidFormat(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"empty", ""},
		{"garbage", "not-a-traceparent"},
		{"wrong version length", "0-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"},
		{"short trace id", "00-0af765-b7ad6b7169203331-01"},
		{"short parent id", "00-0af7651916cd43dd8448eb211c80319c-b7ad-01"},
		{"uppercase", "00-0AF7651916CD43DD8448EB211C80319C-B7AD6B7169203331-01"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := make(http.Header)
			if tt.value != "" {
				h.Set(TraceparentHeader, tt.value)
			}

			_, span := ExtractHTTP(context.Background(), h, "test")

			// Should fall back to new root span.
			if span.TraceID == "" {
				t.Error("should have a trace ID even with invalid input")
			}
		})
	}
}

func TestRoundtripInjectExtract(t *testing.T) {
	// Start a span, inject into headers, extract on the other side.
	ctx, parentSpan := Start(context.Background(), "client-call")

	h := make(http.Header)
	InjectHTTP(ctx, h)

	// Simulate server receiving the request.
	_, serverSpan := ExtractHTTP(context.Background(), h, "server-handle")

	// Server span should share the trace ID.
	if serverSpan.TraceID != parentSpan.TraceID {
		t.Errorf("trace IDs differ: parent=%s, server=%s", parentSpan.TraceID, serverSpan.TraceID)
	}

	// Server span's parent should reference the client span.
	expectedParent := parentSpan.SpanID
	if len(expectedParent) > 16 {
		expectedParent = expectedParent[len(expectedParent)-16:]
	}
	if serverSpan.ParentID != expectedParent {
		t.Errorf("parent ID = %s, want %s", serverSpan.ParentID, expectedParent)
	}
}

func TestTracestateHeader(t *testing.T) {
	ctx, _ := Start(context.Background(), "test")

	h := make(http.Header)
	InjectHTTP(ctx, h)

	// Should set tracestate with mist vendor.
	ts := h.Get(TracestateHeader)
	if ts == "" {
		t.Error("tracestate header should be set")
	}
	if !strings.HasPrefix(ts, "mist=") {
		t.Errorf("tracestate = %s, want mist= prefix", ts)
	}
}

func TestExtractTracestatePreserved(t *testing.T) {
	h := make(http.Header)
	h.Set(TraceparentHeader, "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")
	h.Set(TracestateHeader, "vendor=value,mist=old")

	_, span := ExtractHTTP(context.Background(), h, "test")

	// The extracted span should preserve tracestate from incoming request.
	attrs := span.Attrs()
	ts, ok := attrs["tracestate"].(string)
	if !ok {
		t.Fatal("tracestate not preserved in attrs")
	}
	if !strings.Contains(ts, "vendor=value") {
		t.Errorf("tracestate should contain vendor=value, got %s", ts)
	}
}

func TestParseTraceparent(t *testing.T) {
	tests := []struct {
		input string
		valid bool
		trace string
		span  string
	}{
		{"00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01", true, "0af7651916cd43dd8448eb211c80319c", "b7ad6b7169203331"},
		{"00-00000000000000000000000000000000-b7ad6b7169203331-01", false, "", ""}, // zero trace
		{"00-0af7651916cd43dd8448eb211c80319c-0000000000000000-01", false, "", ""}, // zero parent
		{"", false, "", ""},
		{"garbage", false, "", ""},
	}

	for _, tt := range tests {
		traceID, spanID, ok := ParseTraceparent(tt.input)
		if ok != tt.valid {
			t.Errorf("ParseTraceparent(%q): ok=%v, want %v", tt.input, ok, tt.valid)
		}
		if ok {
			if traceID != tt.trace {
				t.Errorf("ParseTraceparent(%q): trace=%s, want %s", tt.input, traceID, tt.trace)
			}
			if spanID != tt.span {
				t.Errorf("ParseTraceparent(%q): span=%s, want %s", tt.input, spanID, tt.span)
			}
		}
	}
}

func TestFormatTraceparent(t *testing.T) {
	tp := FormatTraceparent("0af7651916cd43dd8448eb211c80319c", "b7ad6b7169203331")
	want := "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
	if tp != want {
		t.Errorf("FormatTraceparent = %s, want %s", tp, want)
	}
}
