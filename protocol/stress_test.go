package protocol

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// TestStressConcurrentMessageCreation verifies that message IDs remain unique
// when many goroutines create messages simultaneously.
func TestStressConcurrentMessageCreation(t *testing.T) {
	const goroutines = 100
	const msgsPerGoroutine = 1000

	ids := make(chan string, goroutines*msgsPerGoroutine)
	var wg sync.WaitGroup

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < msgsPerGoroutine; i++ {
				msg, err := New(SourceMatchSpec, TypeHealthPing, HealthPing{
					From: fmt.Sprintf("goroutine-%d", g),
				})
				if err != nil {
					t.Errorf("goroutine %d, msg %d: New failed: %v", g, i, err)
					return
				}
				ids <- msg.ID
			}
		}(g)
	}

	wg.Wait()
	close(ids)

	seen := make(map[string]bool, goroutines*msgsPerGoroutine)
	for id := range ids {
		if seen[id] {
			t.Fatalf("duplicate ID: %s", id)
		}
		seen[id] = true
	}

	total := goroutines * msgsPerGoroutine
	if len(seen) != total {
		t.Errorf("expected %d unique IDs, got %d", total, len(seen))
	}
}

// TestStressLargePayload tests marshal/unmarshal with payloads approaching 1MB.
func TestStressLargePayload(t *testing.T) {
	sizes := []int{
		1 * 1024,    // 1KB
		64 * 1024,   // 64KB
		256 * 1024,  // 256KB
		512 * 1024,  // 512KB
		1024 * 1024, // 1MB
	}

	for _, size := range sizes {
		t.Run(fmt.Sprintf("%dKB", size/1024), func(t *testing.T) {
			content := strings.Repeat("x", size)
			msg, err := New(SourceInferMux, TypeInferResponse, InferResponse{
				Model:        "test-model",
				Provider:     "test",
				Content:      content,
				TokensIn:     100,
				TokensOut:    int64(size / 4),
				CostUSD:      0.01,
				LatencyMS:    500,
				FinishReason: "stop",
			})
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			data, err := msg.Marshal()
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}

			restored, err := Unmarshal(data)
			if err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			var resp InferResponse
			if err := restored.Decode(&resp); err != nil {
				t.Fatalf("Decode: %v", err)
			}

			if len(resp.Content) != size {
				t.Errorf("content length = %d, want %d", len(resp.Content), size)
			}
			if resp.Content != content {
				t.Error("content corrupted after roundtrip")
			}
		})
	}
}

// TestStressHighVolumeRoundtrip verifies data integrity across thousands of
// marshal/unmarshal cycles with varied payloads.
func TestStressHighVolumeRoundtrip(t *testing.T) {
	const count = 10000

	for i := 0; i < count; i++ {
		span := TraceSpan{
			TraceID:   fmt.Sprintf("trace-%d", i),
			SpanID:    fmt.Sprintf("span-%d", i),
			ParentID:  fmt.Sprintf("parent-%d", i%100),
			Operation: fmt.Sprintf("op-%d", i%10),
			StartNS:   int64(i * 1000000),
			EndNS:     int64(i*1000000 + 500000),
			Status:    "ok",
			Attrs: map[string]any{
				"iter":   float64(i),
				"model":  "test",
				"tokens": float64(i % 4096),
			},
		}

		msg, err := New(SourceTokenTrace, TypeTraceSpan, span)
		if err != nil {
			t.Fatalf("iter %d: New: %v", i, err)
		}

		data, err := msg.Marshal()
		if err != nil {
			t.Fatalf("iter %d: Marshal: %v", i, err)
		}

		restored, err := Unmarshal(data)
		if err != nil {
			t.Fatalf("iter %d: Unmarshal: %v", i, err)
		}

		var decoded TraceSpan
		if err := restored.Decode(&decoded); err != nil {
			t.Fatalf("iter %d: Decode: %v", i, err)
		}

		if decoded.TraceID != span.TraceID {
			t.Fatalf("iter %d: TraceID = %q, want %q", i, decoded.TraceID, span.TraceID)
		}
		if decoded.StartNS != span.StartNS {
			t.Fatalf("iter %d: StartNS = %d, want %d", i, decoded.StartNS, span.StartNS)
		}
	}
}

// TestStressConcurrentMarshalUnmarshal runs marshal/unmarshal from many
// goroutines to verify there's no shared state corruption.
func TestStressConcurrentMarshalUnmarshal(t *testing.T) {
	const goroutines = 50
	const iterations = 500

	msg, _ := New(SourceMatchSpec, TypeEvalResult, EvalResult{
		Suite:      "stress",
		Task:       "concurrent",
		Passed:     true,
		Score:      0.95,
		Baseline:   0.80,
		Delta:      0.15,
		DurationMS: 100,
	})
	data, _ := msg.Marshal()

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				restored, err := Unmarshal(data)
				if err != nil {
					t.Errorf("Unmarshal: %v", err)
					return
				}
				var result EvalResult
				if err := restored.Decode(&result); err != nil {
					t.Errorf("Decode: %v", err)
					return
				}
				if result.Score != 0.95 {
					t.Errorf("Score = %v, want 0.95", result.Score)
					return
				}

				// Marshal back out and verify.
				redata, err := restored.Marshal()
				if err != nil {
					t.Errorf("re-Marshal: %v", err)
					return
				}
				if len(redata) == 0 {
					t.Error("re-Marshal produced empty data")
					return
				}
			}
		}()
	}
	wg.Wait()
}

// TestStressAllPayloadTypes verifies every payload type roundtrips correctly.
func TestStressAllPayloadTypes(t *testing.T) {
	cases := []struct {
		name    string
		source  string
		typ     string
		payload any
		verify  func(t *testing.T, msg *Message)
	}{
		{
			name:    "InferRequest",
			source:  SourceMatchSpec,
			typ:     TypeInferRequest,
			payload: InferRequest{Model: "test", Messages: []ChatMessage{{Role: "user", Content: "hello"}}},
			verify: func(t *testing.T, msg *Message) {
				var v InferRequest
				if err := msg.Decode(&v); err != nil {
					t.Fatal(err)
				}
				if v.Model != "test" || len(v.Messages) != 1 {
					t.Error("InferRequest mismatch")
				}
			},
		},
		{
			name:    "InferResponse",
			source:  SourceInferMux,
			typ:     TypeInferResponse,
			payload: InferResponse{Model: "test", Content: "world", TokensIn: 10, TokensOut: 20, CostUSD: 0.001},
			verify: func(t *testing.T, msg *Message) {
				var v InferResponse
				if err := msg.Decode(&v); err != nil {
					t.Fatal(err)
				}
				if v.Content != "world" || v.TokensOut != 20 {
					t.Error("InferResponse mismatch")
				}
			},
		},
		{
			name:    "EvalRun",
			source:  SourceMatchSpec,
			typ:     TypeEvalRun,
			payload: EvalRun{Suite: "math", Tasks: []string{"add", "mul"}, Baseline: true},
			verify: func(t *testing.T, msg *Message) {
				var v EvalRun
				if err := msg.Decode(&v); err != nil {
					t.Fatal(err)
				}
				if v.Suite != "math" || len(v.Tasks) != 2 || !v.Baseline {
					t.Error("EvalRun mismatch")
				}
			},
		},
		{
			name:    "EvalResult",
			source:  SourceMatchSpec,
			typ:     TypeEvalResult,
			payload: EvalResult{Suite: "math", Task: "add", Passed: true, Score: 1.0, Delta: 0.2},
			verify: func(t *testing.T, msg *Message) {
				var v EvalResult
				if err := msg.Decode(&v); err != nil {
					t.Fatal(err)
				}
				if !v.Passed || v.Score != 1.0 {
					t.Error("EvalResult mismatch")
				}
			},
		},
		{
			name:   "TraceSpan",
			source: SourceTokenTrace,
			typ:    TypeTraceSpan,
			payload: TraceSpan{
				TraceID: "t1", SpanID: "s1", Operation: "inference",
				StartNS: 1000, EndNS: 2000, Status: "ok",
				Attrs: map[string]any{"model": "test", "tokens_in": float64(100)},
			},
			verify: func(t *testing.T, msg *Message) {
				var v TraceSpan
				if err := msg.Decode(&v); err != nil {
					t.Fatal(err)
				}
				if v.TraceID != "t1" || v.Attrs["model"] != "test" {
					t.Error("TraceSpan mismatch")
				}
			},
		},
		{
			name:    "TraceAlert",
			source:  SourceTokenTrace,
			typ:     TypeTraceAlert,
			payload: TraceAlert{Level: "critical", Metric: "error_rate", Value: 0.15, Threshold: 0.05, Message: "error rate high"},
			verify: func(t *testing.T, msg *Message) {
				var v TraceAlert
				if err := msg.Decode(&v); err != nil {
					t.Fatal(err)
				}
				if v.Level != "critical" || v.Value != 0.15 {
					t.Error("TraceAlert mismatch")
				}
			},
		},
		{
			name:    "DataEntities",
			source:  SourceSchemaFlux,
			typ:     TypeDataEntities,
			payload: DataEntities{Count: 5000, Format: "jsonl", Path: "/data/entities.jsonl", Checksum: "sha256:abc123"},
			verify: func(t *testing.T, msg *Message) {
				var v DataEntities
				if err := msg.Decode(&v); err != nil {
					t.Fatal(err)
				}
				if v.Count != 5000 || v.Checksum != "sha256:abc123" {
					t.Error("DataEntities mismatch")
				}
			},
		},
		{
			name:   "DataSchema",
			source: SourceSchemaFlux,
			typ:    TypeDataSchema,
			payload: DataSchema{
				Name: "user",
				Fields: []SchemaField{
					{Name: "id", Type: "int", Required: true},
					{Name: "name", Type: "string", Required: true},
					{Name: "email", Type: "string", Required: false},
				},
			},
			verify: func(t *testing.T, msg *Message) {
				var v DataSchema
				if err := msg.Decode(&v); err != nil {
					t.Fatal(err)
				}
				if v.Name != "user" || len(v.Fields) != 3 {
					t.Error("DataSchema mismatch")
				}
			},
		},
		{
			name:    "HealthPing",
			source:  SourceMatchSpec,
			typ:     TypeHealthPing,
			payload: HealthPing{From: "test"},
			verify: func(t *testing.T, msg *Message) {
				var v HealthPing
				if err := msg.Decode(&v); err != nil {
					t.Fatal(err)
				}
				if v.From != "test" {
					t.Error("HealthPing mismatch")
				}
			},
		},
		{
			name:    "HealthPong",
			source:  SourceInferMux,
			typ:     TypeHealthPong,
			payload: HealthPong{From: "infermux", Version: "1.0.0", Uptime: 3600},
			verify: func(t *testing.T, msg *Message) {
				var v HealthPong
				if err := msg.Decode(&v); err != nil {
					t.Fatal(err)
				}
				if v.Uptime != 3600 || v.Version != "1.0.0" {
					t.Error("HealthPong mismatch")
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg, err := New(tc.source, tc.typ, tc.payload)
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			data, err := msg.Marshal()
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}

			restored, err := Unmarshal(data)
			if err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			if restored.Source != tc.source || restored.Type != tc.typ {
				t.Errorf("envelope mismatch: %s/%s", restored.Source, restored.Type)
			}

			tc.verify(t, restored)
		})
	}
}

// TestStressUnicodePayload verifies that unicode content survives roundtrips.
func TestStressUnicodePayload(t *testing.T) {
	contents := []string{
		"Hello, 世界! 🌍",
		"日本語テスト: こんにちは",
		"Ñoño señor — em-dash «guillemets»",
		"العربية: مرحبا بالعالم",
		"한국어: 안녕하세요",
		"Emoji chain: 🔥💯🚀✨🎯",
		strings.Repeat("émojis 🎭 ", 500),
		"null bytes won't happen in JSON but zero-width: \u200B\u200C\u200D",
	}

	for i, content := range contents {
		t.Run(fmt.Sprintf("unicode_%d", i), func(t *testing.T) {
			msg, err := New(SourceInferMux, TypeInferResponse, InferResponse{
				Content: content,
			})
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			data, err := msg.Marshal()
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}

			restored, err := Unmarshal(data)
			if err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			var resp InferResponse
			if err := restored.Decode(&resp); err != nil {
				t.Fatalf("Decode: %v", err)
			}

			if resp.Content != content {
				t.Errorf("content corrupted: got %q, want %q", resp.Content, content)
			}
		})
	}
}

// TestStressJSONEdgeCases ensures messages handle tricky JSON correctly.
func TestStressJSONEdgeCases(t *testing.T) {
	tricky := map[string]any{
		"empty_string":  "",
		"backslashes":   `path\to\file`,
		"quotes":        `she said "hello"`,
		"newlines":      "line1\nline2\nline3",
		"tabs":          "col1\tcol2\tcol3",
		"html_chars":    "<script>alert('xss')</script>",
		"ampersand":     "a&b&c",
		"large_number":  float64(9999999999999),
		"negative":      float64(-42),
		"zero":          float64(0),
		"nested_object": map[string]any{"deep": map[string]any{"deeper": "value"}},
		"empty_array":   []any{},
		"mixed_array":   []any{"string", float64(42), true, nil},
	}

	msg, err := New(SourceMatchSpec, TypeEvalRun, tricky)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	data, err := msg.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	restored, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	var decoded map[string]any
	if err := restored.Decode(&decoded); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if decoded["backslashes"] != tricky["backslashes"] {
		t.Errorf("backslashes: %q", decoded["backslashes"])
	}
	if decoded["quotes"] != tricky["quotes"] {
		t.Errorf("quotes: %q", decoded["quotes"])
	}
	if decoded["html_chars"] != tricky["html_chars"] {
		t.Errorf("html_chars: %q", decoded["html_chars"])
	}

	nested, ok := decoded["nested_object"].(map[string]any)
	if !ok {
		t.Fatal("nested_object should be a map")
	}
	deep, ok := nested["deep"].(map[string]any)
	if !ok {
		t.Fatal("deep should be a map")
	}
	if deep["deeper"] != "value" {
		t.Errorf("nested value: %v", deep["deeper"])
	}
}

// TestStressMarshalSizeOverhead measures JSON overhead vs raw payload size.
func TestStressMarshalSizeOverhead(t *testing.T) {
	sizes := []int{100, 1000, 10000, 100000}

	for _, size := range sizes {
		t.Run(fmt.Sprintf("%d_bytes", size), func(t *testing.T) {
			payload := InferResponse{Content: strings.Repeat("a", size)}
			raw, _ := json.Marshal(payload)

			msg, _ := New(SourceInferMux, TypeInferResponse, payload)
			envelope, _ := msg.Marshal()

			overhead := float64(len(envelope)-len(raw)) / float64(len(raw)) * 100
			t.Logf("payload=%d, envelope=%d, overhead=%.1f%%", len(raw), len(envelope), overhead)

			// Envelope overhead should be reasonable (< 100% for anything over 100 bytes).
			if size >= 1000 && overhead > 50 {
				t.Errorf("overhead too high: %.1f%%", overhead)
			}
		})
	}
}
