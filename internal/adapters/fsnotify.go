package adapters

import "context"

// ExecFSNotify is the native fsnotify-based watcher used by `awiki
// watchdog`. The stub method returns ErrNotImplemented; the real
// implementation lands in the watchdog slice and depends on
// github.com/fsnotify/fsnotify (added to go.mod by that slice, not
// here).
type ExecFSNotify struct{}

func (ExecFSNotify) Watch(_ context.Context, _ string, _ chan<- string) error {
	return ErrNotImplemented
}
