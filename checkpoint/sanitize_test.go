package checkpoint

import (
	"testing"
)

func TestValidRunIDRejectsPathTraversal(t *testing.T) {
	cases := []struct {
		id   string
		want bool
	}{
		{"valid-run-id", true},
		{"run_123", true},
		{"ABC", true},
		{"../../etc/passwd", false},
		{"../../../tmp/evil", false},
		{"a/b", false},
		{"a\\b", false},
		{"a.b", false},
		{"", false},
		{"a\x00b", false},
		{"a\nb", false},
		{"a b", false},
		{"..", false},
		{".", false},
	}

	for _, tc := range cases {
		if got := ValidRunID(tc.id); got != tc.want {
			t.Errorf("ValidRunID(%q) = %v, want %v", tc.id, got, tc.want)
		}
	}
}

func TestOpenRejectsInvalidRunID(t *testing.T) {
	_, err := Open(t.TempDir(), "../../etc/shadow")
	if err == nil {
		t.Error("expected error for path traversal runID")
	}
}

func TestOpenRejectsEmptyRunID(t *testing.T) {
	_, err := Open(t.TempDir(), "")
	if err == nil {
		t.Error("expected error for empty runID")
	}
}
