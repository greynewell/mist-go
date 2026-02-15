package trace

import (
	"context"
	"testing"

	"github.com/greynewell/mist-go/protocol"
)

// FuzzStartWithTraceID tests that arbitrary trace IDs never cause panics
// and that malicious IDs are sanitized.
func FuzzStartWithTraceID(f *testing.F) {
	f.Add("valid-trace-id", "operation")
	f.Add("", "op")
	f.Add("a\nb\nc", "op")
	f.Add("a\x00b", "op")
	f.Add("<script>alert(1)</script>", "op")
	f.Add(string(make([]byte, 1000)), "op")

	f.Fuzz(func(t *testing.T, traceID, operation string) {
		ctx, span := StartWithTraceID(context.Background(), traceID, operation)
		if span == nil {
			t.Fatal("span should not be nil")
		}
		if span.TraceID == "" {
			t.Error("TraceID should not be empty (should be replaced if invalid)")
		}

		// Verify the trace ID in the context matches the span.
		if TraceID(ctx) != span.TraceID {
			t.Error("context TraceID mismatch")
		}

		// Verify the trace ID is safe (printable ASCII only).
		for _, ch := range span.TraceID {
			if ch < 32 || ch > 126 {
				t.Errorf("trace ID contains non-printable char: %d", ch)
			}
		}
	})
}

// FuzzFromProto tests that arbitrary protocol spans never cause panics.
func FuzzFromProto(f *testing.F) {
	f.Add("t1", "s1", "p1", "op", int64(1000), int64(2000), "ok")
	f.Add("", "", "", "", int64(0), int64(0), "")
	f.Add("a\nb", "c\x00d", "evil", "op\t", int64(-1), int64(-1), "bad\nstatus")

	f.Fuzz(func(t *testing.T, traceID, spanID, parentID, operation string, startNS, endNS int64, status string) {
		ts := protocol.TraceSpan{
			TraceID:   traceID,
			SpanID:    spanID,
			ParentID:  parentID,
			Operation: operation,
			StartNS:   startNS,
			EndNS:     endNS,
			Status:    status,
		}

		span := FromProto(ts)
		if span == nil {
			t.Fatal("FromProto should never return nil")
		}

		// Round-trip.
		proto := span.ToProto()
		_ = proto
	})
}

// FuzzValidID tests that ValidID correctly rejects dangerous inputs.
func FuzzValidID(f *testing.F) {
	f.Add("abc123")
	f.Add("")
	f.Add("a\nb")
	f.Add("a\x00b")
	f.Add(string(make([]byte, 300)))

	f.Fuzz(func(t *testing.T, id string) {
		valid := ValidID(id)
		if valid {
			for _, ch := range id {
				if ch < 32 || ch > 126 {
					t.Errorf("ValidID(%q) = true but contains non-printable", id)
				}
			}
			if len(id) > 256 {
				t.Error("ValidID accepted too-long ID")
			}
		}
	})
}
