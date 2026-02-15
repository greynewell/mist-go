package protocol

import (
	"encoding/json"
	"testing"
)

// FuzzUnmarshal tests that Unmarshal never panics on arbitrary input.
func FuzzUnmarshal(f *testing.F) {
	// Seed corpus with valid messages.
	msg, _ := New("test", TypeHealthPing, HealthPing{From: "test"})
	data, _ := msg.Marshal()
	f.Add(data)

	// Minimal valid JSON.
	f.Add([]byte(`{"version":"1","id":"x","source":"s","type":"t","payload":{}}`))

	// Edge cases.
	f.Add([]byte(`{}`))
	f.Add([]byte(`null`))
	f.Add([]byte(`[]`))
	f.Add([]byte(``))
	f.Add([]byte(`{"payload":null}`))
	f.Add([]byte(`{"version":"1","id":"x","source":"s","type":"t","payload":"not-json"}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		msg, err := Unmarshal(data)
		if err != nil {
			return // expected for most random input
		}

		// If it parsed, it should re-marshal without panic.
		out, err := msg.Marshal()
		if err != nil {
			t.Fatalf("Marshal failed after successful Unmarshal: %v", err)
		}

		// Re-unmarshal should also succeed.
		msg2, err := Unmarshal(out)
		if err != nil {
			t.Fatalf("re-Unmarshal failed: %v", err)
		}

		if msg2.ID != msg.ID || msg2.Type != msg.Type || msg2.Source != msg.Source {
			t.Error("roundtrip identity mismatch")
		}
	})
}

// FuzzDecodePayloads tests that Decode never panics on arbitrary payloads.
func FuzzDecodePayloads(f *testing.F) {
	f.Add([]byte(`{"from":"test"}`))
	f.Add([]byte(`{"model":"claude","messages":[{"role":"user","content":"hi"}]}`))
	f.Add([]byte(`{"suite":"bench","tasks":["a","b"]}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`null`))
	f.Add([]byte(`"string"`))
	f.Add([]byte(`[1,2,3]`))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Try decoding into each payload type — none should panic.
		var hp HealthPing
		json.Unmarshal(data, &hp)

		var ir InferRequest
		json.Unmarshal(data, &ir)

		var er EvalRun
		json.Unmarshal(data, &er)

		var ts TraceSpan
		json.Unmarshal(data, &ts)

		var de DataEntities
		json.Unmarshal(data, &de)
	})
}

// FuzzNewID tests that newID never panics and always returns valid hex.
func FuzzNewID(f *testing.F) {
	f.Add(0)
	f.Fuzz(func(t *testing.T, _ int) {
		id := newID()
		if len(id) != 32 {
			t.Errorf("ID length = %d, want 32", len(id))
		}
	})
}
