package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/greynewell/mist-go/trace"
)

func TestNewLogger(t *testing.T) {
	var buf bytes.Buffer
	log := New("matchspec", LevelInfo, WithWriter(&buf), WithFormat("json"))

	log.Info(context.Background(), "started", "port", 8080)

	if !strings.Contains(buf.String(), `"tool":"matchspec"`) {
		t.Errorf("expected tool field, got: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "started") {
		t.Errorf("expected message, got: %s", buf.String())
	}
}

func TestLogLevels(t *testing.T) {
	var buf bytes.Buffer
	log := New("test", LevelInfo, WithWriter(&buf), WithFormat("json"))

	// Debug should be filtered out.
	log.Debug(context.Background(), "debug msg")
	if buf.Len() > 0 {
		t.Error("debug should be filtered at info level")
	}

	// Info should pass.
	log.Info(context.Background(), "info msg")
	if buf.Len() == 0 {
		t.Error("info should not be filtered at info level")
	}
	buf.Reset()

	// Warn should pass.
	log.Warn(context.Background(), "warn msg")
	if buf.Len() == 0 {
		t.Error("warn should not be filtered at info level")
	}
	buf.Reset()

	// Error should pass.
	log.Error(context.Background(), "error msg")
	if buf.Len() == 0 {
		t.Error("error should not be filtered at info level")
	}
}

func TestSetLevel(t *testing.T) {
	var buf bytes.Buffer
	log := New("test", LevelError, WithWriter(&buf), WithFormat("json"))

	// Info should be filtered at error level.
	log.Info(context.Background(), "should not appear")
	if buf.Len() > 0 {
		t.Error("info should be filtered at error level")
	}

	// Lower the level dynamically.
	log.SetLevel(LevelDebug)
	log.Debug(context.Background(), "now visible")
	if buf.Len() == 0 {
		t.Error("debug should pass after SetLevel(Debug)")
	}
}

func TestTraceContext(t *testing.T) {
	var buf bytes.Buffer
	log := New("test", LevelInfo, WithWriter(&buf), WithFormat("json"))

	ctx, span := trace.Start(context.Background(), "test-op")
	log.Info(ctx, "with trace")

	output := buf.String()
	if !strings.Contains(output, span.TraceID) {
		t.Errorf("expected trace_id %s in output: %s", span.TraceID, output)
	}
	if !strings.Contains(output, span.SpanID) {
		t.Errorf("expected span_id %s in output: %s", span.SpanID, output)
	}
}

func TestNoTraceContext(t *testing.T) {
	var buf bytes.Buffer
	log := New("test", LevelInfo, WithWriter(&buf), WithFormat("json"))

	log.Info(context.Background(), "no trace")

	if strings.Contains(buf.String(), "trace_id") {
		t.Error("should not include trace_id without trace context")
	}
}

func TestWith(t *testing.T) {
	var buf bytes.Buffer
	log := New("test", LevelInfo, WithWriter(&buf), WithFormat("json"))
	child := log.With("request_id", "req-123")

	child.Info(context.Background(), "hello")

	if !strings.Contains(buf.String(), "req-123") {
		t.Errorf("expected request_id in output: %s", buf.String())
	}
}

func TestJSONFormat(t *testing.T) {
	var buf bytes.Buffer
	log := New("test", LevelInfo, WithWriter(&buf), WithFormat("json"))

	log.Info(context.Background(), "test msg", "key", "value")

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if parsed["msg"] != "test msg" {
		t.Errorf("msg = %v", parsed["msg"])
	}
	if parsed["key"] != "value" {
		t.Errorf("key = %v", parsed["key"])
	}
}

func TestTextFormat(t *testing.T) {
	var buf bytes.Buffer
	log := New("test", LevelInfo, WithWriter(&buf), WithFormat("text"))

	log.Info(context.Background(), "hello world")

	output := buf.String()
	if !strings.Contains(output, "hello world") {
		t.Errorf("expected message in text output: %s", output)
	}
	if !strings.Contains(output, "tool=test") {
		t.Errorf("expected tool in text output: %s", output)
	}
}

func TestSlogInterop(t *testing.T) {
	var buf bytes.Buffer
	log := New("test", LevelInfo, WithWriter(&buf), WithFormat("json"))

	slogger := log.Slog()
	slogger.Info("via slog")

	if !strings.Contains(buf.String(), "via slog") {
		t.Error("Slog() should return working slog.Logger")
	}
}
