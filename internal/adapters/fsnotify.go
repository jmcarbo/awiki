package adapters

import (
	"context"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

// ExecFSNotify is the native fsnotify-based watcher used by `awiki
// watchdog`. It wraps github.com/fsnotify/fsnotify.Watcher and emits
// the absolute path of every Create/Write event on the events channel.
// Other events (Remove, Rename, Chmod) are still tracked internally so
// the watcher state stays consistent, but they do not surface on the
// events channel — debounce and stability checks belong in the consumer
// (internal/ingest/watchdog.go).
//
// The watcher recurses into subdirectories on its own: when a Create
// event fires for a directory, the adapter calls watcher.Add on it so
// nested files are picked up without the consumer having to re-walk the
// tree.
//
// Watch closes its watcher when ctx is cancelled and returns nil. Any
// underlying watcher error (e.g. permission denied on Add) is returned
// directly so the consumer can fall through to the polling fallback.
type ExecFSNotify struct{}

// Watch starts an fsnotify watcher rooted at root and forwards
// Create/Write events to the events channel. The events channel is the
// caller's responsibility — Watch never closes it. The function returns
// when ctx is cancelled or the underlying watcher loop errors.
func (ExecFSNotify) Watch(ctx context.Context, root string, events chan<- string) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()

	// Add the root and every existing subdirectory so the watcher sees
	// nested files. fsnotify is non-recursive on most platforms.
	if err := addRecursive(w, root); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-w.Events:
			if !ok {
				return nil
			}
			// On Create of a directory, recursively add it so nested
			// files are tracked. fsnotify emits Create when a directory
			// is moved into the watched tree.
			if ev.Op&fsnotify.Create == fsnotify.Create {
				if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
					_ = addRecursive(w, ev.Name)
					continue
				}
			}
			// Forward only Create+Write file events. Remove/Rename/Chmod
			// are dropped here; the watcher still consumed them so its
			// internal state stays consistent.
			if ev.Op&(fsnotify.Create|fsnotify.Write) == 0 {
				continue
			}
			info, err := os.Stat(ev.Name)
			if err != nil || info.IsDir() {
				continue
			}
			abs := ev.Name
			if !filepath.IsAbs(abs) {
				if a, err := filepath.Abs(abs); err == nil {
					abs = a
				}
			}
			select {
			case <-ctx.Done():
				return nil
			case events <- abs:
			}
		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			if err != nil {
				return err
			}
		}
	}
}

// addRecursive adds root and every existing subdirectory under it to w.
// Files do not need to be added explicitly — fsnotify reports events
// for files inside a watched directory.
func addRecursive(w *fsnotify.Watcher, root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip unreadable subtrees but keep walking the rest.
			return nil
		}
		if info.IsDir() {
			// Add is idempotent for already-watched paths.
			_ = w.Add(path)
		}
		return nil
	})
}
