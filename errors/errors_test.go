package errors

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestNew(t *testing.T) {
	err := New(CodeValidation, "bad input")
	if err.Code != CodeValidation {
		t.Errorf("Code = %q", err.Code)
	}
	if err.Message != "bad input" {
		t.Errorf("Message = %q", err.Message)
	}
}

func TestNewf(t *testing.T) {
	err := Newf(CodeTimeout, "timed out after %ds", 30)
	if err.Message != "timed out after 30s" {
		t.Errorf("Message = %q", err.Message)
	}
}

func TestWrap(t *testing.T) {
	cause := fmt.Errorf("connection refused")
	err := Wrap(CodeTransport, cause, "send failed")

	if err.Cause != cause {
		t.Error("cause not set")
	}
	if err.Code != CodeTransport {
		t.Errorf("Code = %q", err.Code)
	}
	expected := "transport: send failed: connection refused"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}

func TestWrapNilCause(t *testing.T) {
	err := Wrap(CodeTransport, nil, "should be nil")
	if err != nil {
		t.Error("Wrap(nil) should return nil")
	}
}

func TestWrapf(t *testing.T) {
	cause := fmt.Errorf("timeout")
	err := Wrapf(CodeTimeout, cause, "after %d attempts", 3)
	if err.Message != "after 3 attempts" {
		t.Errorf("Message = %q", err.Message)
	}
}

func TestWithMeta(t *testing.T) {
	err := New(CodeNotFound, "entity missing").
		WithMeta("entity_id", "abc123").
		WithMeta("entity_type", "user")

	if err.Meta["entity_id"] != "abc123" {
		t.Error("missing entity_id meta")
	}
	if err.Meta["entity_type"] != "user" {
		t.Error("missing entity_type meta")
	}
}

func TestWithMetaDoesNotMutateOriginal(t *testing.T) {
	original := New(CodeValidation, "test")
	modified := original.WithMeta("key", "value")

	if original.Meta != nil {
		t.Error("original should not have meta")
	}
	if modified.Meta["key"] != "value" {
		t.Error("modified should have meta")
	}
}

func TestUnwrap(t *testing.T) {
	cause := fmt.Errorf("root cause")
	err := Wrap(CodeInternal, cause, "wrapper")

	if err.Unwrap() != cause {
		t.Error("Unwrap should return cause")
	}
}

func TestErrorString(t *testing.T) {
	err := New(CodeAuth, "invalid token")
	if err.Error() != "auth: invalid token" {
		t.Errorf("Error() = %q", err.Error())
	}
}

func TestErrorStringWithCause(t *testing.T) {
	err := Wrap(CodeTransport, fmt.Errorf("EOF"), "read failed")
	if err.Error() != "transport: read failed: EOF" {
		t.Errorf("Error() = %q", err.Error())
	}
}

func TestMarshalJSON(t *testing.T) {
	err := Wrap(CodeValidation, fmt.Errorf("field required"), "bad request").
		WithMeta("field", "name")

	data, jsonErr := json.Marshal(err)
	if jsonErr != nil {
		t.Fatalf("MarshalJSON: %v", jsonErr)
	}

	var decoded map[string]any
	json.Unmarshal(data, &decoded)

	if decoded["code"] != "validation" {
		t.Errorf("code = %v", decoded["code"])
	}
	if decoded["cause"] != "field required" {
		t.Errorf("cause = %v", decoded["cause"])
	}
	meta, _ := decoded["meta"].(map[string]any)
	if meta["field"] != "name" {
		t.Errorf("meta.field = %v", meta["field"])
	}
}

func TestMarshalJSONNoCause(t *testing.T) {
	err := New(CodeNotFound, "not found")
	data, _ := json.Marshal(err)

	var decoded map[string]any
	json.Unmarshal(data, &decoded)

	if _, ok := decoded["cause"]; ok {
		t.Error("cause should be omitted when nil")
	}
}

func TestCodeExtraction(t *testing.T) {
	err := New(CodeTimeout, "slow")
	if Code(err) != CodeTimeout {
		t.Errorf("Code() = %q", Code(err))
	}
}

func TestCodeExtractionWrapped(t *testing.T) {
	inner := New(CodeRateLimit, "too fast")
	outer := fmt.Errorf("wrapped: %w", inner)
	if Code(outer) != CodeRateLimit {
		t.Errorf("Code() = %q, want rate_limit", Code(outer))
	}
}

func TestCodeExtractionNonMIST(t *testing.T) {
	err := fmt.Errorf("plain error")
	if Code(err) != CodeInternal {
		t.Errorf("Code() = %q, want internal", Code(err))
	}
}

func TestCodeNil(t *testing.T) {
	if Code(nil) != "" {
		t.Error("Code(nil) should be empty")
	}
}

func TestHTTPStatus(t *testing.T) {
	cases := []struct {
		code   string
		status int
	}{
		{CodeValidation, http.StatusBadRequest},
		{CodeAuth, http.StatusUnauthorized},
		{CodeNotFound, http.StatusNotFound},
		{CodeConflict, http.StatusConflict},
		{CodeRateLimit, http.StatusTooManyRequests},
		{CodeTimeout, http.StatusGatewayTimeout},
		{CodeUnavailable, http.StatusServiceUnavailable},
		{CodeCancelled, 499},
		{CodeInternal, http.StatusInternalServerError},
		{CodeTransport, http.StatusInternalServerError},
		{CodeProtocol, http.StatusInternalServerError},
		{"unknown", http.StatusInternalServerError},
	}

	for _, tc := range cases {
		if got := HTTPStatus(tc.code); got != tc.status {
			t.Errorf("HTTPStatus(%q) = %d, want %d", tc.code, got, tc.status)
		}
	}
}

func TestExitCode(t *testing.T) {
	cases := []struct {
		code string
		exit int
	}{
		{CodeValidation, 2},
		{CodeNotFound, 3},
		{CodeAuth, 4},
		{CodeTimeout, 5},
		{CodeCancelled, 130},
		{CodeInternal, 1},
		{"unknown", 1},
	}

	for _, tc := range cases {
		if got := ExitCode(tc.code); got != tc.exit {
			t.Errorf("ExitCode(%q) = %d, want %d", tc.code, got, tc.exit)
		}
	}
}

func TestIsAndAs(t *testing.T) {
	inner := New(CodeValidation, "bad")
	outer := fmt.Errorf("outer: %w", inner)

	var target *Error
	if !As(outer, &target) {
		t.Fatal("As should find *Error in chain")
	}
	if target.Code != CodeValidation {
		t.Errorf("Code = %q", target.Code)
	}
}

func TestAsNonMIST(t *testing.T) {
	var target *Error
	if As(fmt.Errorf("plain"), &target) {
		t.Error("As should return false for non-MIST errors")
	}
}

func TestIsRetryableTransient(t *testing.T) {
	cases := []struct {
		code string
		want bool
	}{
		{CodeTimeout, true},
		{CodeTransport, true},
		{CodeUnavailable, true},
		{CodeRateLimit, true},
		{CodeValidation, false},
		{CodeAuth, false},
		{CodeNotFound, false},
		{CodeConflict, false},
		{CodeProtocol, false},
		{CodeInternal, false},
	}
	for _, tc := range cases {
		err := New(tc.code, "test")
		if got := IsRetryable(err); got != tc.want {
			t.Errorf("IsRetryable(%q) = %v, want %v", tc.code, got, tc.want)
		}
	}
}

func TestIsRetryableExplicitOverride(t *testing.T) {
	// Mark a normally-permanent error as retryable.
	err := New(CodeValidation, "flaky validation").Retriable()
	if !IsRetryable(err) {
		t.Error("explicitly retryable error should be retryable")
	}

	// Mark a normally-transient error as permanent.
	err2 := New(CodeTimeout, "permanent timeout").Permanent()
	if IsRetryable(err2) {
		t.Error("explicitly permanent error should not be retryable")
	}
}

func TestIsRetryableNonMIST(t *testing.T) {
	err := fmt.Errorf("generic error")
	if !IsRetryable(err) {
		t.Error("non-MIST errors should be assumed retryable")
	}
}

func TestIsRetryableNil(t *testing.T) {
	if IsRetryable(nil) {
		t.Error("nil should not be retryable")
	}
}

func TestRetriableDoesNotMutate(t *testing.T) {
	original := New(CodeValidation, "test")
	retryable := original.Retriable()

	if IsRetryable(original) {
		t.Error("original should not be retryable")
	}
	if !IsRetryable(retryable) {
		t.Error("retryable copy should be retryable")
	}
}

func TestAllCodesAreUnique(t *testing.T) {
	codes := []string{
		CodeInternal, CodeTimeout, CodeCancelled, CodeTransport,
		CodeProtocol, CodeValidation, CodeNotFound, CodeUnavailable,
		CodeRateLimit, CodeAuth, CodeConflict,
	}
	seen := make(map[string]bool)
	for _, c := range codes {
		if seen[c] {
			t.Errorf("duplicate code: %s", c)
		}
		seen[c] = true
	}
}
