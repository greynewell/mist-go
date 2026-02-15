package protocol

import (
	"fmt"
	"strconv"
	"strings"
)

// Version constants for the MIST protocol.
const (
	// CurrentVersion is the protocol version used by this library.
	CurrentVersion = "1"

	// MinSupportedVersion is the oldest version this library can read.
	MinSupportedVersion = "1"

	// MaxSupportedVersion is the newest version this library understands.
	MaxSupportedVersion = "1"
)

// CheckVersion validates that a message version is compatible with this
// library. Returns nil if compatible, an error describing the issue otherwise.
func CheckVersion(version string) error {
	if version == "" {
		return fmt.Errorf("protocol: empty version")
	}

	v, err := parseVersion(version)
	if err != nil {
		return fmt.Errorf("protocol: invalid version %q: %w", version, err)
	}

	minV, _ := parseVersion(MinSupportedVersion)
	maxV, _ := parseVersion(MaxSupportedVersion)

	if v < minV {
		return fmt.Errorf("protocol: version %q is too old (min supported: %s)", version, MinSupportedVersion)
	}
	if v > maxV {
		return fmt.Errorf("protocol: version %q is too new (max supported: %s)", version, MaxSupportedVersion)
	}

	return nil
}

// NegotiateVersion finds the highest compatible version between a local
// and remote version range. Returns the negotiated version string or an
// error if no compatible version exists.
//
// Each side provides its supported range as "min-max" (e.g., "1-3")
// or a single version (e.g., "1" meaning "1-1").
func NegotiateVersion(local, remote string) (string, error) {
	localMin, localMax, err := parseRange(local)
	if err != nil {
		return "", fmt.Errorf("protocol: invalid local version range %q: %w", local, err)
	}

	remoteMin, remoteMax, err := parseRange(remote)
	if err != nil {
		return "", fmt.Errorf("protocol: invalid remote version range %q: %w", remote, err)
	}

	// Find the overlap.
	overlapMin := localMin
	if remoteMin > overlapMin {
		overlapMin = remoteMin
	}
	overlapMax := localMax
	if remoteMax < overlapMax {
		overlapMax = remoteMax
	}

	if overlapMin > overlapMax {
		return "", fmt.Errorf("protocol: no compatible version (local %s, remote %s)", local, remote)
	}

	// Use the highest compatible version.
	return strconv.Itoa(overlapMax), nil
}

// IsCompatible checks whether a message version falls within the supported
// range of this library.
func IsCompatible(version string) bool {
	return CheckVersion(version) == nil
}

// VersionInfo returns a human-readable string describing the supported
// version range.
func VersionInfo() string {
	if MinSupportedVersion == MaxSupportedVersion {
		return fmt.Sprintf("MIST protocol version %s", CurrentVersion)
	}
	return fmt.Sprintf("MIST protocol version %s (supports %s-%s)",
		CurrentVersion, MinSupportedVersion, MaxSupportedVersion)
}

func parseVersion(s string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(s))
}

func parseRange(s string) (min, max int, err error) {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "-"); idx >= 0 {
		min, err = parseVersion(s[:idx])
		if err != nil {
			return 0, 0, err
		}
		max, err = parseVersion(s[idx+1:])
		if err != nil {
			return 0, 0, err
		}
		if min > max {
			return 0, 0, fmt.Errorf("min %d > max %d", min, max)
		}
		return min, max, nil
	}

	v, err := parseVersion(s)
	if err != nil {
		return 0, 0, err
	}
	return v, v, nil
}
