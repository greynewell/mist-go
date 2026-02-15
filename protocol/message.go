// Package protocol defines the MIST message envelope and types used for
// all inter-tool communication. Messages are serialized as JSON and
// carried over any transport (HTTP, file, stdio, or in-process channels).
package protocol

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"time"
)

// MaxMessageSize is the maximum allowed size of a serialized message (10 MB).
// This prevents resource exhaustion from oversized payloads.
const MaxMessageSize = 10 << 20

// Message types for inter-tool communication.
const (
	// Data pipeline (SchemaFlux)
	TypeDataEntities = "data.entities" // batch of compiled entities
	TypeDataSchema   = "data.schema"   // schema definition

	// Inference (InferMux)
	TypeInferRequest  = "infer.request"  // LLM inference request
	TypeInferResponse = "infer.response" // LLM inference response

	// Evaluation (MatchSpec)
	TypeEvalRun    = "eval.run"    // start an evaluation
	TypeEvalResult = "eval.result" // evaluation outcome

	// Observability (TokenTrace)
	TypeTraceSpan  = "trace.span"  // a single trace span
	TypeTraceAlert = "trace.alert" // quality/cost/latency alert

	// Health (all tools)
	TypeHealthPing = "health.ping"
	TypeHealthPong = "health.pong"
)

// Source identifiers for MIST tools.
const (
	SourceSchemaFlux = "schemaflux"
	SourceInferMux   = "infermux"
	SourceMatchSpec  = "matchspec"
	SourceTokenTrace = "tokentrace"
)

// Message is the universal envelope for all MIST inter-tool communication.
type Message struct {
	Version     string          `json:"version"`
	ID          string          `json:"id"`
	Source      string          `json:"source"`
	Type        string          `json:"type"`
	TimestampNS int64           `json:"timestamp_ns"`
	Payload     json.RawMessage `json:"payload"`
	Checksum    uint32          `json:"checksum,omitempty"`
}

// New creates a message with a random ID and current timestamp.
func New(source, typ string, payload any) (*Message, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &Message{
		Version:     "1",
		ID:          newID(),
		Source:      source,
		Type:        typ,
		TimestampNS: time.Now().UnixNano(),
		Payload:     raw,
	}, nil
}

// Validate checks that the message envelope has the required fields.
func (m *Message) Validate() error {
	if m.Version == "" {
		return fmt.Errorf("message: missing version")
	}
	if m.ID == "" {
		return fmt.Errorf("message: missing id")
	}
	if m.Source == "" {
		return fmt.Errorf("message: missing source")
	}
	if m.Type == "" {
		return fmt.Errorf("message: missing type")
	}
	if len(m.Payload) > MaxMessageSize {
		return fmt.Errorf("message: payload too large: %d bytes", len(m.Payload))
	}
	return nil
}

// Decode unmarshals the payload into the given value.
func (m *Message) Decode(v any) error {
	return json.Unmarshal(m.Payload, v)
}

// ComputeChecksum sets the CRC32 checksum based on the current payload.
// Call this before Marshal to include integrity verification.
func (m *Message) ComputeChecksum() {
	m.Checksum = crc32.ChecksumIEEE(m.Payload)
}

// VerifyChecksum returns true if the checksum is valid or absent (zero).
// A zero checksum means integrity checking was not enabled for this message.
func (m *Message) VerifyChecksum() bool {
	if m.Checksum == 0 {
		return true // not set, skip check
	}
	return m.Checksum == crc32.ChecksumIEEE(m.Payload)
}

// Marshal serializes the message to JSON bytes.
func (m *Message) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

// MarshalWithChecksum computes the checksum and serializes the message.
func (m *Message) MarshalWithChecksum() ([]byte, error) {
	m.ComputeChecksum()
	return json.Marshal(m)
}

// Unmarshal deserializes a message from JSON bytes.
// Returns an error if the data exceeds MaxMessageSize.
func Unmarshal(data []byte) (*Message, error) {
	if len(data) > MaxMessageSize {
		return nil, fmt.Errorf("message too large: %d bytes (max %d)", len(data), MaxMessageSize)
	}
	var m Message
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("mist: crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
