package errors

import (
	"encoding/json"
	"fmt"
	"testing"
)

// FuzzErrorMarshalUnmarshal tests that Error JSON round-trips never panic.
func FuzzErrorMarshalUnmarshal(f *testing.F) {
	f.Add("internal", "test error", "key", "value")
	f.Add("validation", "bad input", "", "")
	f.Add("", "", "", "")
	f.Add("timeout", "slow\nwith\nnewlines", "path", "/etc/passwd")
	f.Add("auth", "token: <script>alert('xss')</script>", "html", "<b>bold</b>")

	f.Fuzz(func(t *testing.T, code, message, metaKey, metaValue string) {
		err := New(code, message)
		if metaKey != "" {
			err = err.WithMeta(metaKey, metaValue)
		}

		// Must never panic.
		_ = err.Error()

		data, jsonErr := json.Marshal(err)
		if jsonErr != nil {
			return
		}

		// Unmarshal into a map (not back into Error since Cause is complex).
		var decoded map[string]any
		json.Unmarshal(data, &decoded)
	})
}

// FuzzIsRetryable tests that IsRetryable never panics.
func FuzzIsRetryable(f *testing.F) {
	f.Add("internal")
	f.Add("timeout")
	f.Add("validation")
	f.Add("")
	f.Add("unknown_code")

	f.Fuzz(func(t *testing.T, code string) {
		err := New(code, "test")
		_ = IsRetryable(err)
		_ = IsRetryable(err.Retriable())
		_ = IsRetryable(err.Permanent())

		// Wrapped error.
		wrapped := fmt.Errorf("outer: %w", err)
		_ = IsRetryable(wrapped)
	})
}
