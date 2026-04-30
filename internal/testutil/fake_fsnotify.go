package testutil

import (
	"context"
	"errors"
	"sync"

	"awiki/internal/adapters"
)

// FakeFSNotify is a test double for adapters.FSNotify. Tests push paths
// onto the In channel; the adapter forwards them to the events channel
// passed to Watch. Set FailInit to a non-nil error to simulate fsnotify
// init failure (the adapter returns the error before consuming any
// events; the watchdog falls through to the polling backend).
//
// Watch returns nil on ctx cancellation. The In channel is closed by
// the test (or left open and drained on cancellation) — Watch never
// closes it.
type FakeFSNotify struct {
	// FailInit, when non-nil, makes Watch return immediately with this
	// error. Used to drive the fsnotify-init-failed branch in watchdog.
	FailInit error

	// In is the test-driven event source. Tests push paths into this
	// channel; the adapter forwards them to the events channel passed
	// to Watch.
	In chan string

	mu       sync.Mutex
	watching bool
}

// Watch forwards In events to the events channel until ctx is
// cancelled. When FailInit is non-nil, Watch returns immediately with
// that error.
func (f *FakeFSNotify) Watch(ctx context.Context, _ string, events chan<- string) error {
	if f.FailInit != nil {
		return f.FailInit
	}
	if f.In == nil {
		// No source channel; block until ctx cancel so the watchdog
		// has a "live" watcher to time out against.
		<-ctx.Done()
		return nil
	}
	f.mu.Lock()
	f.watching = true
	f.mu.Unlock()
	for {
		select {
		case <-ctx.Done():
			return nil
		case path, ok := <-f.In:
			if !ok {
				return nil
			}
			select {
			case events <- path:
			case <-ctx.Done():
				return nil
			}
		}
	}
}

// Watching reports whether Watch is currently sourcing events. Used by
// tests to confirm the adapter is live before pushing to In.
func (f *FakeFSNotify) Watching() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.watching
}

// ErrFSNotifyInit is a sentinel error tests can pass via FailInit to
// drive the fsnotify-init-failed fall-through.
var ErrFSNotifyInit = errors.New("fake fsnotify: init failed")

// Compile-time interface check.
var _ adapters.FSNotify = (*FakeFSNotify)(nil)
