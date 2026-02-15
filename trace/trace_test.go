package trace

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/greynewell/mist-go/protocol"
)

func TestStartCreatesSpan(t *testing.T) {
	ctx, span := Start(context.Background(), "test-op")

	if span.TraceID == "" {
		t.Error("TraceID should not be empty")
	}
	if span.SpanID == "" {
		t.Error("SpanID should not be empty")
	}
	if span.ParentID != "" {
		t.Error("root span should have no parent")
	}
	if span.Operation != "test-op" {
		t.Errorf("Operation = %q", span.Operation)
	}
	if span.StartNS == 0 {
		t.Error("StartNS should not be zero")
	}

	// Context should carry the span.
	got := FromContext(ctx)
	if got != span {
		t.Error("span not found in context")
	}
}

func TestStartInheritsTraceID(t *testing.T) {
	ctx, parent := Start(context.Background(), "parent")
	_, child := Start(ctx, "child")

	if child.TraceID != parent.TraceID {
		t.Error("child should inherit parent's trace ID")
	}
	if child.ParentID != parent.SpanID {
		t.Error("child's parent ID should be parent's span ID")
	}
	if child.SpanID == parent.SpanID {
		t.Error("child should have its own span ID")
	}
}

func TestStartDeepNesting(t *testing.T) {
	ctx := context.Background()
	var spans []*Span

	for i := 0; i < 10; i++ {
		var span *Span
		ctx, span = Start(ctx, "level")
		spans = append(spans, span)
	}

	// All should share the same trace ID.
	for _, s := range spans {
		if s.TraceID != spans[0].TraceID {
			t.Error("all spans should share trace ID")
		}
	}

	// Each should point to the previous.
	for i := 1; i < len(spans); i++ {
		if spans[i].ParentID != spans[i-1].SpanID {
			t.Errorf("span %d parent should be span %d", i, i-1)
		}
	}
}

func TestStartWithTraceID(t *testing.T) {
	traceID := "explicit-trace-id"
	ctx, span := StartWithTraceID(context.Background(), traceID, "op")

	if span.TraceID != traceID {
		t.Errorf("TraceID = %q, want %q", span.TraceID, traceID)
	}

	// Child should inherit explicit trace ID.
	_, child := Start(ctx, "child")
	if child.TraceID != traceID {
		t.Error("child should inherit explicit trace ID")
	}
}

func TestEnd(t *testing.T) {
	_, span := Start(context.Background(), "op")
	time.Sleep(time.Millisecond)
	span.End("ok")

	if span.Status != "ok" {
		t.Errorf("Status = %q", span.Status)
	}
	if span.EndNS == 0 {
		t.Error("EndNS should not be zero")
	}
	if span.EndNS <= span.StartNS {
		t.Error("EndNS should be after StartNS")
	}
}

func TestDuration(t *testing.T) {
	_, span := Start(context.Background(), "op")
	time.Sleep(5 * time.Millisecond)
	span.End("ok")

	ns := span.DurationNS()
	if ns < 5_000_000 { // at least 5ms
		t.Errorf("DurationNS = %d, expected >= 5ms", ns)
	}

	ms := span.DurationMS()
	if ms < 5.0 {
		t.Errorf("DurationMS = %f, expected >= 5.0", ms)
	}
}

func TestDurationNotEnded(t *testing.T) {
	_, span := Start(context.Background(), "op")
	if span.DurationNS() != 0 {
		t.Error("unended span should have zero duration")
	}
}

func TestSetAttr(t *testing.T) {
	_, span := Start(context.Background(), "op")
	span.SetAttr("model", "claude-sonnet-4-5-20250929")
	span.SetAttr("tokens_out", 500)
	span.SetAttr("cost_usd", 0.003)

	attrs := span.Attrs()
	if attrs["model"] != "claude-sonnet-4-5-20250929" {
		t.Errorf("model = %v", attrs["model"])
	}
	if attrs["tokens_out"] != 500 {
		t.Errorf("tokens_out = %v", attrs["tokens_out"])
	}
}

func TestAttrsConcurrent(t *testing.T) {
	_, span := Start(context.Background(), "op")
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			span.SetAttr("key", i)
			span.Attrs() // concurrent read
		}(i)
	}
	wg.Wait()
}

func TestAttrsReturnsACopy(t *testing.T) {
	_, span := Start(context.Background(), "op")
	span.SetAttr("key", "original")

	attrs := span.Attrs()
	attrs["key"] = "modified"

	if span.Attrs()["key"] != "original" {
		t.Error("Attrs should return a copy")
	}
}

func TestFromContextNil(t *testing.T) {
	span := FromContext(context.Background())
	if span != nil {
		t.Error("expected nil span from bare context")
	}
}

func TestTraceIDFromContext(t *testing.T) {
	ctx, span := Start(context.Background(), "op")
	if TraceID(ctx) != span.TraceID {
		t.Error("TraceID should match span")
	}
	if TraceID(context.Background()) != "" {
		t.Error("TraceID from bare context should be empty")
	}
}

func TestSpanIDFromContext(t *testing.T) {
	ctx, span := Start(context.Background(), "op")
	if SpanID(ctx) != span.SpanID {
		t.Error("SpanID should match span")
	}
}

func TestNewIDUniqueness(t *testing.T) {
	seen := make(map[string]bool, 10000)
	for i := 0; i < 10000; i++ {
		id := NewID()
		if seen[id] {
			t.Fatalf("duplicate ID: %s", id)
		}
		seen[id] = true
	}
}

func TestToProto(t *testing.T) {
	_, span := Start(context.Background(), "inference")
	span.SetAttr("model", "test")
	span.End("ok")

	proto := span.ToProto()
	if proto.TraceID != span.TraceID {
		t.Error("TraceID mismatch")
	}
	if proto.Operation != "inference" {
		t.Error("Operation mismatch")
	}
	if proto.Status != "ok" {
		t.Error("Status mismatch")
	}
	if proto.Attrs["model"] != "test" {
		t.Error("Attrs mismatch")
	}
}

func TestFromProto(t *testing.T) {
	ts := protocol.TraceSpan{
		TraceID:   "t1",
		SpanID:    "s1",
		ParentID:  "p1",
		Operation: "inference",
		StartNS:   1000,
		EndNS:     2000,
		Status:    "ok",
		Attrs:     map[string]any{"model": "test"},
	}

	span := FromProto(ts)
	if span.TraceID != "t1" || span.Operation != "inference" {
		t.Error("FromProto field mismatch")
	}
	if span.Attrs()["model"] != "test" {
		t.Error("Attrs mismatch")
	}
}

func TestFromProtoNilAttrs(t *testing.T) {
	ts := protocol.TraceSpan{TraceID: "t1", SpanID: "s1"}
	span := FromProto(ts)
	if span.Attrs() == nil {
		t.Error("Attrs should not be nil")
	}
}

func TestContinueFrom(t *testing.T) {
	ts := protocol.TraceSpan{
		TraceID:   "incoming-trace",
		SpanID:    "incoming-span",
		Operation: "parent-op",
	}

	ctx, child := ContinueFrom(context.Background(), ts, "child-op")
	if child.TraceID != "incoming-trace" {
		t.Error("should inherit trace ID")
	}
	if child.ParentID != "incoming-span" {
		t.Error("parent should be incoming span")
	}
	if child.Operation != "child-op" {
		t.Error("operation mismatch")
	}
	if FromContext(ctx) != child {
		t.Error("child should be in context")
	}
}

func TestSpanToMessage(t *testing.T) {
	_, span := Start(context.Background(), "inference")
	span.SetAttr("model", "test")
	span.End("ok")

	msg, err := SpanToMessage("matchspec", span)
	if err != nil {
		t.Fatalf("SpanToMessage: %v", err)
	}
	if msg.Type != protocol.TypeTraceSpan {
		t.Errorf("Type = %q", msg.Type)
	}
	if msg.Source != "matchspec" {
		t.Errorf("Source = %q", msg.Source)
	}

	var decoded protocol.TraceSpan
	if err := msg.Decode(&decoded); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded.TraceID != span.TraceID {
		t.Error("decoded trace ID mismatch")
	}
}

func TestRoundtripProto(t *testing.T) {
	_, original := Start(context.Background(), "roundtrip")
	original.SetAttr("model", "claude")
	original.SetAttr("tokens", 500)
	original.End("ok")

	proto := original.ToProto()
	restored := FromProto(proto)

	if restored.TraceID != original.TraceID {
		t.Error("TraceID mismatch")
	}
	if restored.Operation != original.Operation {
		t.Error("Operation mismatch")
	}
	if restored.Attrs()["model"] != "claude" {
		t.Error("Attrs mismatch")
	}
}
