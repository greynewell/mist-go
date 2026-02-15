package checkpoint

import (
	"testing"
)

// FuzzReplay tests that replaying arbitrary checkpoint data never panics.
func FuzzReplay(f *testing.F) {
	// Valid checkpoint lines.
	f.Add(`{"step":"download","status":"completed","timestamp":"2024-01-01T00:00:00Z","result":"ok"}
{"step":"process","status":"running","timestamp":"2024-01-01T00:01:00Z"}
`)
	// Corrupted data.
	f.Add(`not json at all
{"step":"ok","status":"completed"}
{broken
`)
	// Empty.
	f.Add("")
	f.Add(`{}`)
	f.Add(`null`)

	f.Fuzz(func(t *testing.T, data string) {
		tracker := &Tracker{
			completed: make(map[string]*Record),
			results:   make(map[string]any),
		}
		// Must never panic.
		tracker.replay([]byte(data))
	})
}

// FuzzValidRunID tests that ValidRunID correctly rejects dangerous inputs.
func FuzzValidRunID(f *testing.F) {
	f.Add("valid-run-id")
	f.Add("run_123")
	f.Add("../../etc/passwd")
	f.Add("../../../tmp/evil")
	f.Add("")
	f.Add("a")
	f.Add("a/b")
	f.Add("a\x00b")
	f.Add("a\nb")

	f.Fuzz(func(t *testing.T, id string) {
		valid := ValidRunID(id)

		if valid {
			// If valid, must not contain path separators or control chars.
			for _, ch := range id {
				if ch == '/' || ch == '\\' || ch == '.' || ch < 32 {
					t.Errorf("ValidRunID(%q) = true, but contains dangerous char %q", id, ch)
				}
			}
			if id == "" {
				t.Error("ValidRunID('') = true")
			}
		}
	})
}
