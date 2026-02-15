// Package errors provides structured error types for the MIST stack.
// Every error has a code, a human message, and optional metadata.
// Codes map to HTTP status codes and process exit codes so tools
// behave consistently whether run as APIs or CLI commands.
package errors

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Standard error codes used across all MIST tools.
const (
	CodeInternal    = "internal"     // unexpected failure
	CodeTimeout     = "timeout"      // operation timed out
	CodeCancelled   = "cancelled"    // context was cancelled
	CodeTransport   = "transport"    // send/receive failure
	CodeProtocol    = "protocol"     // malformed message or envelope
	CodeValidation  = "validation"   // invalid input or config
	CodeNotFound    = "not_found"    // resource does not exist
	CodeUnavailable = "unavailable"  // service is down or unreachable
	CodeRateLimit   = "rate_limit"   // too many requests
	CodeAuth        = "auth"         // authentication or authorization failure
	CodeConflict    = "conflict"     // resource conflict or version mismatch
)

// Error is a structured error that carries a code, message, causal chain,
// and optional metadata. It implements the error and json.Marshaler interfaces.
type Error struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Cause   error             `json:"-"`
	Meta    map[string]string `json:"meta,omitempty"`
	// retryOverride: nil = use default for code, ptr to true/false = explicit.
	retryOverride *bool
}

// New creates an error with the given code and message.
func New(code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// Newf creates an error with a formatted message.
func Newf(code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Wrap wraps a cause error with a MIST code and message.
// If cause is nil, returns nil.
func Wrap(code string, cause error, message string) *Error {
	if cause == nil {
		return nil
	}
	return &Error{Code: code, Message: message, Cause: cause}
}

// Wrapf wraps a cause error with a MIST code and formatted message.
func Wrapf(code string, cause error, format string, args ...any) *Error {
	if cause == nil {
		return nil
	}
	return &Error{Code: code, Message: fmt.Sprintf(format, args...), Cause: cause}
}

// WithMeta returns a copy of the error with additional metadata.
func (e *Error) WithMeta(key, value string) *Error {
	cp := *e
	if cp.Meta == nil {
		cp.Meta = make(map[string]string)
	}
	cp.Meta[key] = value
	return &cp
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the cause for errors.Is/As chains.
func (e *Error) Unwrap() error {
	return e.Cause
}

// MarshalJSON serializes the error including the cause message.
func (e *Error) MarshalJSON() ([]byte, error) {
	type alias Error
	aux := struct {
		*alias
		Cause string `json:"cause,omitempty"`
	}{alias: (*alias)(e)}
	if e.Cause != nil {
		aux.Cause = e.Cause.Error()
	}
	return json.Marshal(aux)
}

// retryableCodes are error codes that indicate a transient failure
// which may succeed on retry.
var retryableCodes = map[string]bool{
	CodeTimeout:     true,
	CodeTransport:   true,
	CodeUnavailable: true,
	CodeRateLimit:   true,
}

// IsRetryable reports whether an error is worth retrying.
// For *Error types, it checks for an explicit override first (via Retriable/Permanent),
// then falls back to whether the code is in the retryable set.
// For non-MIST errors, returns true (assume transient).
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	var e *Error
	if As(err, &e) {
		// Explicit override takes priority.
		if e.retryOverride != nil {
			return *e.retryOverride
		}
		return retryableCodes[e.Code]
	}
	// Unknown errors are assumed transient.
	return true
}

// Retriable returns a copy of the error explicitly marked as retryable.
func (e *Error) Retriable() *Error {
	cp := *e
	t := true
	cp.retryOverride = &t
	return &cp
}

// Permanent returns a copy of the error explicitly marked as non-retryable.
func (e *Error) Permanent() *Error {
	cp := *e
	f := false
	cp.retryOverride = &f
	return &cp
}

// Code extracts the MIST error code from any error. If the error is not
// a *Error, returns CodeInternal.
func Code(err error) string {
	if err == nil {
		return ""
	}
	var e *Error
	if As(err, &e) {
		return e.Code
	}
	return CodeInternal
}

// HTTPStatus maps a MIST error code to an HTTP status code.
func HTTPStatus(code string) int {
	switch code {
	case CodeValidation:
		return http.StatusBadRequest
	case CodeAuth:
		return http.StatusUnauthorized
	case CodeNotFound:
		return http.StatusNotFound
	case CodeConflict:
		return http.StatusConflict
	case CodeRateLimit:
		return http.StatusTooManyRequests
	case CodeTimeout:
		return http.StatusGatewayTimeout
	case CodeUnavailable:
		return http.StatusServiceUnavailable
	case CodeCancelled:
		return 499 // Client Closed Request
	case CodeTransport, CodeProtocol, CodeInternal:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// ExitCode maps a MIST error code to a process exit code.
func ExitCode(code string) int {
	switch code {
	case CodeValidation:
		return 2
	case CodeNotFound:
		return 3
	case CodeAuth:
		return 4
	case CodeTimeout:
		return 5
	case CodeUnavailable:
		return 6
	case CodeTransport:
		return 7
	case CodeProtocol:
		return 8
	case CodeRateLimit:
		return 9
	case CodeConflict:
		return 10
	case CodeCancelled:
		return 130 // 128 + SIGINT
	default:
		return 1
	}
}

// Is reports whether any error in err's chain matches target.
// This is a convenience re-export of the standard library function
// so callers don't need to import both packages.
func Is(err, target error) bool {
	if err == nil || target == nil {
		return err == target
	}
	for {
		if err == target {
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
		if err == nil {
			return false
		}
	}
}

// As finds the first error in err's chain that matches target.
func As(err error, target any) bool {
	if err == nil {
		return false
	}
	// Use type assertion for our own Error type.
	if t, ok := target.(**Error); ok {
		for {
			if e, ok := err.(*Error); ok {
				*t = e
				return true
			}
			u, ok := err.(interface{ Unwrap() error })
			if !ok {
				return false
			}
			err = u.Unwrap()
			if err == nil {
				return false
			}
		}
	}
	return false
}
