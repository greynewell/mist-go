package protocol

import (
	"testing"
)

func TestNewMessage(t *testing.T) {
	payload := HealthPing{From: "matchspec"}
	msg, err := New(SourceMatchSpec, TypeHealthPing, payload)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if msg.Version != "1" {
		t.Errorf("Version = %q, want 1", msg.Version)
	}
	if msg.Source != "matchspec" {
		t.Errorf("Source = %q", msg.Source)
	}
	if msg.Type != "health.ping" {
		t.Errorf("Type = %q", msg.Type)
	}
	if msg.ID == "" {
		t.Error("ID should not be empty")
	}
	if msg.TimestampNS == 0 {
		t.Error("TimestampNS should not be zero")
	}
}

func TestMessageIDsAreUnique(t *testing.T) {
	msg1, _ := New(SourceInferMux, TypeHealthPing, HealthPing{From: "a"})
	msg2, _ := New(SourceInferMux, TypeHealthPing, HealthPing{From: "b"})

	if msg1.ID == msg2.ID {
		t.Error("two messages should have different IDs")
	}
}

func TestMarshalUnmarshal(t *testing.T) {
	original, err := New(SourceSchemaFlux, TypeDataEntities, DataEntities{
		Count:  100,
		Format: "jsonl",
		Path:   "/tmp/data.jsonl",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	data, err := original.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	restored, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if restored.ID != original.ID {
		t.Errorf("ID mismatch: %q != %q", restored.ID, original.ID)
	}
	if restored.Source != original.Source {
		t.Errorf("Source mismatch")
	}
	if restored.Type != original.Type {
		t.Errorf("Type mismatch")
	}
}

func TestDecode(t *testing.T) {
	msg, err := New(SourceTokenTrace, TypeTraceSpan, TraceSpan{
		TraceID:   "t1",
		SpanID:    "s1",
		Operation: "inference",
		StartNS:   1000,
		EndNS:     2000,
		Status:    "ok",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var span TraceSpan
	if err := msg.Decode(&span); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if span.TraceID != "t1" {
		t.Errorf("TraceID = %q", span.TraceID)
	}
	if span.Operation != "inference" {
		t.Errorf("Operation = %q", span.Operation)
	}
	if span.EndNS != 2000 {
		t.Errorf("EndNS = %d", span.EndNS)
	}
}

func TestDecodeInferRequest(t *testing.T) {
	req := InferRequest{
		Model: "claude-sonnet-4-5-20250929",
		Messages: []ChatMessage{
			{Role: "user", Content: "hello"},
		},
		Params: map[string]any{"temperature": 0.7},
	}
	msg, err := New(SourceMatchSpec, TypeInferRequest, req)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var decoded InferRequest
	if err := msg.Decode(&decoded); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if decoded.Model != "claude-sonnet-4-5-20250929" {
		t.Errorf("Model = %q", decoded.Model)
	}
	if len(decoded.Messages) != 1 || decoded.Messages[0].Content != "hello" {
		t.Errorf("Messages = %v", decoded.Messages)
	}
}

func TestDecodeEvalResult(t *testing.T) {
	result := EvalResult{
		Suite:    "math",
		Task:     "addition",
		Passed:   true,
		Score:    0.95,
		Baseline: 0.80,
		Delta:    0.15,
	}
	msg, err := New(SourceMatchSpec, TypeEvalResult, result)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var decoded EvalResult
	if err := msg.Decode(&decoded); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if decoded.Delta != 0.15 {
		t.Errorf("Delta = %v", decoded.Delta)
	}
}

func TestUnmarshalInvalidJSON(t *testing.T) {
	_, err := Unmarshal([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestMessageTypes(t *testing.T) {
	types := []string{
		TypeDataEntities, TypeDataSchema,
		TypeInferRequest, TypeInferResponse,
		TypeEvalRun, TypeEvalResult,
		TypeTraceSpan, TypeTraceAlert,
		TypeHealthPing, TypeHealthPong,
	}
	seen := make(map[string]bool)
	for _, typ := range types {
		if seen[typ] {
			t.Errorf("duplicate type: %s", typ)
		}
		seen[typ] = true
	}
}
