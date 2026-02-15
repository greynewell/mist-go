// Package protocol defines the MIST message envelope and types used for
// all inter-tool communication. Messages are serialized as JSON and
// carried over any transport (HTTP, file, stdio, or in-process channels).
package protocol

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"
)

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

// Decode unmarshals the payload into the given value.
func (m *Message) Decode(v any) error {
	return json.Unmarshal(m.Payload, v)
}

// Marshal serializes the message to JSON bytes.
func (m *Message) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

// Unmarshal deserializes a message from JSON bytes.
func Unmarshal(data []byte) (*Message, error) {
	var m Message
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func newID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
