package protocol

import (
	"strings"
	"testing"
)

func BenchmarkNewMessage(b *testing.B) {
	payload := HealthPing{From: "bench"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = New(SourceMatchSpec, TypeHealthPing, payload)
	}
}

func BenchmarkMarshal_Small(b *testing.B) {
	msg, _ := New(SourceMatchSpec, TypeHealthPing, HealthPing{From: "bench"})
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = msg.Marshal()
	}
}

func BenchmarkUnmarshal_Small(b *testing.B) {
	msg, _ := New(SourceMatchSpec, TypeHealthPing, HealthPing{From: "bench"})
	data, _ := msg.Marshal()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = Unmarshal(data)
	}
}

func BenchmarkMarshal_Medium(b *testing.B) {
	msg, _ := New(SourceMatchSpec, TypeInferRequest, InferRequest{
		Model:    "claude-sonnet-4-5-20250929",
		Provider: "anthropic",
		Messages: []ChatMessage{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: strings.Repeat("benchmark input data ", 50)},
		},
		Params: map[string]any{"temperature": 0.7, "max_tokens": 4096},
		Meta:   map[string]string{"trace_id": "abc123", "request_id": "def456"},
	})
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = msg.Marshal()
	}
}

func BenchmarkUnmarshal_Medium(b *testing.B) {
	msg, _ := New(SourceMatchSpec, TypeInferRequest, InferRequest{
		Model:    "claude-sonnet-4-5-20250929",
		Provider: "anthropic",
		Messages: []ChatMessage{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: strings.Repeat("benchmark input data ", 50)},
		},
		Params: map[string]any{"temperature": 0.7, "max_tokens": 4096},
		Meta:   map[string]string{"trace_id": "abc123", "request_id": "def456"},
	})
	data, _ := msg.Marshal()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = Unmarshal(data)
	}
}

func BenchmarkMarshal_Large(b *testing.B) {
	msg, _ := New(SourceInferMux, TypeInferResponse, InferResponse{
		Model:        "claude-sonnet-4-5-20250929",
		Provider:     "anthropic",
		Content:      strings.Repeat("This is a large response body with lots of generated text. ", 1000),
		TokensIn:     150,
		TokensOut:    4096,
		CostUSD:      0.015,
		LatencyMS:    2500,
		FinishReason: "stop",
	})
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = msg.Marshal()
	}
}

func BenchmarkUnmarshal_Large(b *testing.B) {
	msg, _ := New(SourceInferMux, TypeInferResponse, InferResponse{
		Model:        "claude-sonnet-4-5-20250929",
		Provider:     "anthropic",
		Content:      strings.Repeat("This is a large response body with lots of generated text. ", 1000),
		TokensIn:     150,
		TokensOut:    4096,
		CostUSD:      0.015,
		LatencyMS:    2500,
		FinishReason: "stop",
	})
	data, _ := msg.Marshal()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = Unmarshal(data)
	}
}

func BenchmarkMarshalUnmarshal_Roundtrip(b *testing.B) {
	span := TraceSpan{
		TraceID:   "trace-abc123",
		SpanID:    "span-def456",
		ParentID:  "span-parent",
		Operation: "inference",
		StartNS:   1700000000000000000,
		EndNS:     1700000000500000000,
		Status:    "ok",
		Attrs: map[string]any{
			"model":      "claude-sonnet-4-5-20250929",
			"provider":   "anthropic",
			"tokens_in":  150,
			"tokens_out": 500,
			"cost_usd":   0.003,
		},
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		msg, _ := New(SourceTokenTrace, TypeTraceSpan, span)
		data, _ := msg.Marshal()
		restored, _ := Unmarshal(data)
		var decoded TraceSpan
		_ = restored.Decode(&decoded)
	}
}

func BenchmarkDecode_TraceSpan(b *testing.B) {
	msg, _ := New(SourceTokenTrace, TypeTraceSpan, TraceSpan{
		TraceID:   "trace-abc123",
		SpanID:    "span-def456",
		Operation: "inference",
		StartNS:   1700000000000000000,
		EndNS:     1700000000500000000,
		Status:    "ok",
		Attrs: map[string]any{
			"model": "claude-sonnet-4-5-20250929",
			"cost":  0.003,
		},
	})
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var span TraceSpan
		_ = msg.Decode(&span)
	}
}

func BenchmarkNewID(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = newID()
	}
}
