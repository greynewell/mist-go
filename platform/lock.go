package platform

import (
	"fmt"
	"os"
	"path/filepath"
)

// FileLock provides advisory file locking for cross-platform coordination.
// On Unix systems it uses flock(2), on Windows it uses LockFileEx.
//
// Usage:
//
//	lock, err := platform.Lock("/var/run/mist.lock")
//	if err != nil {
//	    log.Fatal("another instance is running")
//	}
//	defer lock.Unlock()
type FileLock struct {
	path string
	f    *os.File
}

// Lock acquires an exclusive lock on the given file path.
// The file is created if it doesn't exist.
// Returns an error if the lock is already held by another process.
func Lock(path string) (*FileLock, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("platform: lock: %w", err)
	}

	// Ensure parent directory exists.
	dir := filepath.Dir(abs)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("platform: lock: mkdir: %w", err)
	}

	f, err := os.OpenFile(abs, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("platform: lock: open: %w", err)
	}

	if err := lockFile(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("platform: lock: %w", err)
	}

	return &FileLock{path: abs, f: f}, nil
}

// TryLock attempts to acquire an exclusive lock without blocking.
// Returns nil, nil if the lock is already held.
func TryLock(path string) (*FileLock, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("platform: trylock: %w", err)
	}

	dir := filepath.Dir(abs)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("platform: trylock: mkdir: %w", err)
	}

	f, err := os.OpenFile(abs, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("platform: trylock: open: %w", err)
	}

	if err := tryLockFile(f); err != nil {
		f.Close()
		return nil, nil // Lock is held by another process.
	}

	return &FileLock{path: abs, f: f}, nil
}

// Unlock releases the file lock and removes the lock file.
func (l *FileLock) Unlock() error {
	if l.f == nil {
		return nil
	}
	unlockFile(l.f)
	err := l.f.Close()
	os.Remove(l.path)
	l.f = nil
	return err
}

// Path returns the lock file path.
func (l *FileLock) Path() string {
	return l.path
}
