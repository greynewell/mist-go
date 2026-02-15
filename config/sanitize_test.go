package config

import (
	"strings"
	"testing"
)

func TestTOMLArraySizeLimit(t *testing.T) {
	// Build a TOML line with more than maxArrayElements elements.
	var b strings.Builder
	b.WriteString("values = [")
	for i := 0; i < maxArrayElements+10; i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("1")
	}
	b.WriteString("]")

	_, err := ParseTOML(strings.NewReader(b.String()))
	if err == nil {
		t.Error("expected error for oversized array")
	}
	if !strings.Contains(err.Error(), "exceeds maximum") {
		t.Errorf("error = %q, want 'exceeds maximum'", err.Error())
	}
}

func TestTOMLHandlesValueAfterComment(t *testing.T) {
	// Value that becomes empty after stripping inline comment.
	input := "key = # just a comment"
	data, err := ParseTOML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data["key"] != "" {
		t.Errorf("expected empty string, got %v", data["key"])
	}
}

func TestTOMLRejectsUnclosedArray(t *testing.T) {
	_, err := ParseTOML(strings.NewReader("key = [1, 2, 3"))
	if err == nil {
		t.Error("expected error for unclosed array")
	}
}

func TestTOMLRejectsUnclosedTable(t *testing.T) {
	_, err := ParseTOML(strings.NewReader("[table"))
	if err == nil {
		t.Error("expected error for unclosed table header")
	}
}
