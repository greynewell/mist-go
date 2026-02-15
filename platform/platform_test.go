package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

func TestOS(t *testing.T) {
	os := OS()
	if os == "" {
		t.Error("OS should not be empty")
	}
	if os != runtime.GOOS {
		t.Errorf("OS = %s, want %s", os, runtime.GOOS)
	}
}

func TestArch(t *testing.T) {
	arch := Arch()
	if arch == "" {
		t.Error("Arch should not be empty")
	}
	if arch != runtime.GOARCH {
		t.Errorf("Arch = %s, want %s", arch, runtime.GOARCH)
	}
}

func TestIsWindows(t *testing.T) {
	want := runtime.GOOS == "windows"
	if IsWindows() != want {
		t.Errorf("IsWindows = %v, want %v", IsWindows(), want)
	}
}

// Line ending tests

func TestNormalizeLineEndings(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"unix", "a\nb\nc\n", "a\nb\nc\n"},
		{"windows", "a\r\nb\r\nc\r\n", "a\nb\nc\n"},
		{"mixed", "a\r\nb\nc\r\n", "a\nb\nc\n"},
		{"no newlines", "abc", "abc"},
		{"bare cr", "a\rb\rc", "a\rb\rc"}, // bare \r left alone
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(NormalizeLineEndings([]byte(tt.input)))
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPlatformLineEnding(t *testing.T) {
	le := PlatformLineEnding()
	if le != "\n" && le != "\r\n" {
		t.Errorf("unexpected line ending: %q", le)
	}
}

func TestToPlatformLineEndings(t *testing.T) {
	input := []byte("a\nb\nc\n")
	got := ToPlatformLineEndings(input)

	if IsWindows() {
		want := "a\r\nb\r\nc\r\n"
		if string(got) != want {
			t.Errorf("got %q, want %q", string(got), want)
		}
	} else {
		// On Unix, should be unchanged.
		if string(got) != string(input) {
			t.Errorf("got %q, want %q", string(got), string(input))
		}
	}
}

// File locking tests

func TestLockUnlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.lock")

	lock, err := Lock(path)
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if lock.Path() != path {
		// Path might be resolved to absolute.
		abs, _ := filepath.Abs(path)
		if lock.Path() != abs {
			t.Errorf("Path = %s, want %s", lock.Path(), abs)
		}
	}

	if err := lock.Unlock(); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
}

func TestTryLockSuccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.lock")

	lock, err := TryLock(path)
	if err != nil {
		t.Fatalf("TryLock: %v", err)
	}
	if lock == nil {
		t.Fatal("expected lock to succeed")
	}
	lock.Unlock()
}

func TestTryLockAlreadyHeld(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.lock")

	lock1, err := Lock(path)
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}
	defer lock1.Unlock()

	lock2, err := TryLock(path)
	if err != nil {
		t.Fatalf("TryLock error: %v", err)
	}
	if lock2 != nil {
		lock2.Unlock()
		t.Error("TryLock should return nil when lock is held")
	}
}

func TestLockReacquireAfterUnlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.lock")

	lock1, _ := Lock(path)
	lock1.Unlock()

	lock2, err := Lock(path)
	if err != nil {
		t.Fatalf("Lock after unlock: %v", err)
	}
	lock2.Unlock()
}

func TestLockCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "test.lock")

	lock, err := Lock(path)
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}
	defer lock.Unlock()

	// File should exist.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("lock file should be created")
	}
}

func TestUnlockIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.lock")
	lock, _ := Lock(path)
	lock.Unlock()
	// Second unlock should not error.
	if err := lock.Unlock(); err != nil {
		t.Errorf("second Unlock: %v", err)
	}
}

func TestShutdownSignals(t *testing.T) {
	sigs := ShutdownSignals()
	if len(sigs) == 0 {
		t.Error("ShutdownSignals should return at least one signal")
	}
}

// Stress tests

func TestLockConcurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.lock")
	var wg sync.WaitGroup
	errors := make(chan error, 100)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lock, err := TryLock(path)
			if err != nil {
				errors <- err
				return
			}
			if lock != nil {
				lock.Unlock()
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent lock error: %v", err)
	}
}

func TestNormalizeLineEndingsLarge(t *testing.T) {
	// 1MB of CRLF text.
	data := make([]byte, 0, 1<<20)
	for len(data) < 1<<20 {
		data = append(data, "line of text\r\n"...)
	}

	got := NormalizeLineEndings(data)
	for i := 0; i < len(got)-1; i++ {
		if got[i] == '\r' && got[i+1] == '\n' {
			t.Fatal("found CRLF after normalization")
		}
	}
}
