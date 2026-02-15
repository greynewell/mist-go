package protocol

import (
	"strings"
	"testing"
)

func TestUnmarshalRejectsOversized(t *testing.T) {
	// Create a message with a payload bigger than MaxMessageSize.
	huge := make([]byte, MaxMessageSize+1)
	for i := range huge {
		huge[i] = 'x'
	}
	_, err := Unmarshal(huge)
	if err == nil {
		t.Error("expected error for oversized message")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("error = %q, want 'too large'", err.Error())
	}
}

func TestUnmarshalRejectsMissingFields(t *testing.T) {
	cases := []struct {
		name string
		json string
	}{
		{"missing version", `{"id":"x","source":"s","type":"t","payload":{}}`},
		{"missing id", `{"version":"1","source":"s","type":"t","payload":{}}`},
		{"missing source", `{"version":"1","id":"x","type":"t","payload":{}}`},
		{"missing type", `{"version":"1","id":"x","source":"s","payload":{}}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Unmarshal([]byte(tc.json))
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestValidateAcceptsValid(t *testing.T) {
	msg, err := New("test", TypeHealthPing, HealthPing{From: "test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := msg.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestNewIDLength(t *testing.T) {
	id := newID()
	if len(id) != 32 {
		t.Errorf("ID length = %d, want 32", len(id))
	}
	// Should be valid hex.
	for _, ch := range id {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
			t.Errorf("ID contains non-hex char: %c", ch)
		}
	}
}
