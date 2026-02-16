package trace

import (
	"context"
	"testing"
)

func TestValidIDRejectsControlChars(t *testing.T) {
	cases := []struct {
		id   string
		want bool
	}{
		{"abc123", true},
		{"abc-def_ghi.jkl", true},
		{"hex0123456789abcdef", true},
		{"", false},
		{"a\nb", false},                    // newline — log injection
		{"a\rb", false},                    // carriage return — log injection
		{"a\x00b", false},                  // null byte
		{"a\tb", false},                    // tab
		{string(make([]byte, 300)), false}, // too long
	}

	for _, tc := range cases {
		if got := ValidID(tc.id); got != tc.want {
			t.Errorf("ValidID(%q) = %v, want %v", tc.id, got, tc.want)
		}
	}
}

func TestStartWithTraceIDSanitizesInvalid(t *testing.T) {
	// Malicious trace ID with newlines (log injection attempt).
	_, span := StartWithTraceID(context.Background(), "evil\ninjected\nlines", "op")

	// Should get a sanitized (replaced) trace ID.
	if span.TraceID == "evil\ninjected\nlines" {
		t.Error("malicious trace ID should be replaced")
	}
	if span.TraceID == "" {
		t.Error("trace ID should not be empty")
	}

	// Verify the replacement is valid hex.
	if !ValidID(span.TraceID) {
		t.Errorf("replacement trace ID is not valid: %q", span.TraceID)
	}
}

func TestStartWithTraceIDAcceptsValid(t *testing.T) {
	_, span := StartWithTraceID(context.Background(), "explicit-trace-123", "op")
	if span.TraceID != "explicit-trace-123" {
		t.Errorf("valid trace ID should be preserved, got %q", span.TraceID)
	}
}
