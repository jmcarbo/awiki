package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

// ContentionError indicates the lock could not be acquired before the
// configured timeout. Exit code 7 matches scripts/lib/lock.sh.
type ContentionError struct {
	Path string
}

func (e *ContentionError) Error() string {
	return fmt.Sprintf("lock contention: %s", e.Path)
}

func (e *ContentionError) ExitCode() int { return 7 }

// WithExclusiveLock acquires an exclusive flock on path, runs body, and
// releases. If the lock is held by another process for longer than
// timeout, returns *ContentionError without running body.
func WithExclusiveLock(path string, timeout time.Duration, body func() error) error {
	return withLock(path, unix.LOCK_EX, timeout, body)
}

// WithSharedLock is the LOCK_SH variant.
func WithSharedLock(path string, timeout time.Duration, body func() error) error {
	return withLock(path, unix.LOCK_SH, timeout, body)
}

func withLock(path string, mode int, timeout time.Duration, body func() error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	deadline := time.Now().Add(timeout)
	for {
		if err := unix.Flock(int(f.Fd()), mode|unix.LOCK_NB); err == nil {
			break
		}
		if time.Now().After(deadline) {
			return &ContentionError{Path: path}
		}
		time.Sleep(20 * time.Millisecond)
	}
	defer unix.Flock(int(f.Fd()), unix.LOCK_UN) //nolint:errcheck
	return body()
}
