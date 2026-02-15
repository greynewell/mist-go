package protocol

import (
	"strings"
	"testing"
)

func TestCheckVersionCurrent(t *testing.T) {
	if err := CheckVersion(CurrentVersion); err != nil {
		t.Errorf("current version should be valid: %v", err)
	}
}

func TestCheckVersionEmpty(t *testing.T) {
	err := CheckVersion("")
	if err == nil {
		t.Error("empty version should be invalid")
	}
}

func TestCheckVersionInvalid(t *testing.T) {
	err := CheckVersion("abc")
	if err == nil {
		t.Error("non-numeric version should be invalid")
	}
}

func TestCheckVersionTooOld(t *testing.T) {
	err := CheckVersion("0")
	if err == nil {
		t.Error("version 0 should be too old")
	}
	if !strings.Contains(err.Error(), "too old") {
		t.Errorf("error = %q, want 'too old'", err.Error())
	}
}

func TestCheckVersionTooNew(t *testing.T) {
	err := CheckVersion("999")
	if err == nil {
		t.Error("version 999 should be too new")
	}
	if !strings.Contains(err.Error(), "too new") {
		t.Errorf("error = %q, want 'too new'", err.Error())
	}
}

func TestIsCompatible(t *testing.T) {
	if !IsCompatible("1") {
		t.Error("version 1 should be compatible")
	}
	if IsCompatible("0") {
		t.Error("version 0 should not be compatible")
	}
	if IsCompatible("999") {
		t.Error("version 999 should not be compatible")
	}
}

func TestNegotiateVersionSame(t *testing.T) {
	v, err := NegotiateVersion("1", "1")
	if err != nil {
		t.Fatalf("NegotiateVersion: %v", err)
	}
	if v != "1" {
		t.Errorf("negotiated = %s, want 1", v)
	}
}

func TestNegotiateVersionRange(t *testing.T) {
	v, err := NegotiateVersion("1-3", "2-5")
	if err != nil {
		t.Fatalf("NegotiateVersion: %v", err)
	}
	// Overlap is 2-3, highest is 3.
	if v != "3" {
		t.Errorf("negotiated = %s, want 3", v)
	}
}

func TestNegotiateVersionNoOverlap(t *testing.T) {
	_, err := NegotiateVersion("1-2", "3-4")
	if err == nil {
		t.Error("expected error for no overlap")
	}
	if !strings.Contains(err.Error(), "no compatible") {
		t.Errorf("error = %q, want 'no compatible'", err.Error())
	}
}

func TestNegotiateVersionSingleAndRange(t *testing.T) {
	v, err := NegotiateVersion("2", "1-3")
	if err != nil {
		t.Fatalf("NegotiateVersion: %v", err)
	}
	if v != "2" {
		t.Errorf("negotiated = %s, want 2", v)
	}
}

func TestNegotiateVersionInvalidLocal(t *testing.T) {
	_, err := NegotiateVersion("abc", "1")
	if err == nil {
		t.Error("expected error for invalid local")
	}
}

func TestNegotiateVersionInvalidRemote(t *testing.T) {
	_, err := NegotiateVersion("1", "abc")
	if err == nil {
		t.Error("expected error for invalid remote")
	}
}

func TestNegotiateVersionInvertedRange(t *testing.T) {
	_, err := NegotiateVersion("3-1", "1")
	if err == nil {
		t.Error("expected error for inverted range (3-1)")
	}
}

func TestVersionInfo(t *testing.T) {
	info := VersionInfo()
	if info == "" {
		t.Error("VersionInfo should not be empty")
	}
	if !strings.Contains(info, CurrentVersion) {
		t.Errorf("VersionInfo = %q, want to contain %s", info, CurrentVersion)
	}
}

func TestMessageVersionValidation(t *testing.T) {
	// Create a message and verify its version is compatible.
	msg, err := New("test", TypeHealthPing, HealthPing{From: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if !IsCompatible(msg.Version) {
		t.Errorf("new message version %s should be compatible", msg.Version)
	}
}
