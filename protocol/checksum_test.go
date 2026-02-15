package protocol

import (
	"testing"
)

func TestChecksumRoundtrip(t *testing.T) {
	msg, _ := New("test", TypeHealthPing, HealthPing{From: "test"})

	data, err := msg.MarshalWithChecksum()
	if err != nil {
		t.Fatalf("MarshalWithChecksum: %v", err)
	}

	decoded, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Checksum == 0 {
		t.Error("checksum should be set")
	}
	if !decoded.VerifyChecksum() {
		t.Error("checksum should verify after roundtrip")
	}
}

func TestChecksumDetectsCorruption(t *testing.T) {
	msg, _ := New("test", TypeHealthPing, HealthPing{From: "test"})
	msg.ComputeChecksum()

	// Corrupt the payload.
	msg.Payload = []byte(`{"from":"corrupted"}`)

	if msg.VerifyChecksum() {
		t.Error("checksum should fail after payload corruption")
	}
}

func TestChecksumAbsentIsValid(t *testing.T) {
	msg, _ := New("test", TypeHealthPing, HealthPing{From: "test"})
	// No checksum computed.
	if !msg.VerifyChecksum() {
		t.Error("absent checksum (zero) should be treated as valid")
	}
}

func TestChecksumDeterministic(t *testing.T) {
	msg, _ := New("test", TypeHealthPing, HealthPing{From: "test"})
	msg.ComputeChecksum()
	c1 := msg.Checksum

	msg.ComputeChecksum()
	c2 := msg.Checksum

	if c1 != c2 {
		t.Errorf("checksum not deterministic: %d != %d", c1, c2)
	}
}

func TestMarshalWithChecksum(t *testing.T) {
	msg, _ := New("test", TypeEvalRun, EvalRun{Suite: "bench", Baseline: true})

	data, err := msg.MarshalWithChecksum()
	if err != nil {
		t.Fatalf("MarshalWithChecksum: %v", err)
	}

	// Should contain checksum field.
	decoded, _ := Unmarshal(data)
	if decoded.Checksum == 0 {
		t.Error("checksum should be non-zero")
	}
}

func TestChecksumLargePayload(t *testing.T) {
	large := make([]byte, 1024*1024) // 1MB
	for i := range large {
		large[i] = byte(i % 256)
	}

	msg, _ := New("test", TypeDataEntities, DataEntities{
		Count:  1000,
		Format: "json",
		Path:   "/data/entities.jsonl",
	})

	msg.ComputeChecksum()
	if msg.Checksum == 0 {
		t.Error("checksum should be non-zero for non-empty payload")
	}
	if !msg.VerifyChecksum() {
		t.Error("checksum should verify")
	}
}
