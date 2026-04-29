package fsutil

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestExclusiveLockReturnsContentionExit(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, ".awiki", "lock")

	// First holder grabs the lock and never releases until done.
	released := make(chan struct{})
	holding := make(chan struct{})
	go func() {
		defer close(released)
		err := WithExclusiveLock(lockPath, time.Second, func() error {
			close(holding)
			time.Sleep(200 * time.Millisecond)
			return nil
		})
		if err != nil {
			t.Errorf("first holder: %v", err)
		}
	}()
	<-holding

	// Second attempt must time out.
	err := WithExclusiveLock(lockPath, 50*time.Millisecond, func() error {
		t.Fatal("body must not run")
		return nil
	})
	var cont *ContentionError
	if !errors.As(err, &cont) {
		t.Fatalf("got %v, want ContentionError", err)
	}
	<-released
}

func TestSharedLockAllowsConcurrentReaders(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, ".awiki", "lock")

	first := make(chan struct{})
	go func() {
		_ = WithSharedLock(lockPath, time.Second, func() error {
			close(first)
			time.Sleep(100 * time.Millisecond)
			return nil
		})
	}()
	<-first

	if err := WithSharedLock(lockPath, 50*time.Millisecond, func() error {
		return nil
	}); err != nil {
		t.Fatalf("shared/shared should not block: %v", err)
	}
}

func TestContentionErrorExitCodeIsSeven(t *testing.T) {
	if (&ContentionError{}).ExitCode() != 7 {
		t.Fatalf("contention exit code must be 7 to match lock.sh")
	}
}
