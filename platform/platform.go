// Package platform provides cross-platform abstractions for MIST tools.
// It handles OS differences in file locking, signal handling, and text
// normalization so tool code can remain platform-agnostic.
package platform

import (
	"bytes"
	"runtime"
)

// OS returns the current operating system name.
func OS() string {
	return runtime.GOOS
}

// Arch returns the current architecture.
func Arch() string {
	return runtime.GOARCH
}

// IsWindows reports whether the current OS is Windows.
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// NormalizeLineEndings converts \r\n to \n for consistent processing.
// This is essential for file transport on Windows where text files may
// use CRLF line endings.
func NormalizeLineEndings(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
}

// PlatformLineEnding returns the native line ending for the current OS.
func PlatformLineEnding() string {
	if IsWindows() {
		return "\r\n"
	}
	return "\n"
}

// ToPlatformLineEndings converts \n to the native line ending.
func ToPlatformLineEndings(data []byte) []byte {
	if !IsWindows() {
		return data
	}
	// First normalize, then convert to CRLF.
	normalized := NormalizeLineEndings(data)
	return bytes.ReplaceAll(normalized, []byte("\n"), []byte("\r\n"))
}
