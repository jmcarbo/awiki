package ingest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// WatchdogOptions parameterizes one `awiki watchdog` invocation. The
// option set is a strict subset of the bash watchdog.sh CLI surface:
// only the verbs the slice 9 plan pins (catchup / once / poll-interval)
// plus DebounceMs and BatchDir/FailedDir overrides for tests.
//
// Field semantics:
//
//   - Catchup       : drain existing files in BatchDir at startup before
//                     entering the watch loop. Mirrors `--catchup`.
//   - Once          : exit after one event-and-process cycle. Used by
//                     tests; mirrors `--once`.
//   - PollInterval  : when > 0, force the polling fallback at the given
//                     interval. When 0, use fsnotify and fall through to
//                     a 2 s poll if fsnotify init fails.
//   - DebounceMs    : per-path debounce window before invoking ingest.
//                     0 → defaults to 500 ms, matching the slice plan.
//   - BatchDir      : override raw/inbox/batch (test injection). When
//                     empty, runner.BatchDir() is used.
//   - FailedDir     : override raw/inbox/batch/_failed (test injection).
//                     When empty, defaults to filepath.Join(BatchDir, "_failed").
type WatchdogOptions struct {
	Catchup      bool
	Once         bool
	PollInterval time.Duration
	DebounceMs   int
	BatchDir     string
	FailedDir    string
}

// IngestSingleFn is the per-file ingest hook the watchdog calls when a
// path settles after debounce. Production wires it to
// (*Runner).IngestSingle; tests inject a stub via the watchdog runner
// helper to drive ingest success/failure paths deterministically.
type IngestSingleFn func(ctx context.Context, path string, stdout, stderr io.Writer) error

// WatchdogRunner bundles the hooks Watchdog needs but that Runner does
// not naturally provide. This stays separate from Runner so the test
// harness can swap IngestSingle without instantiating a full Runner.
type WatchdogRunner struct {
	*Runner
	// IngestSingle is the per-file ingest call. When nil, defaults to
	// (*Runner).IngestSingle.
	IngestSingle IngestSingleFn
}

// Watchdog implements the watch loop driven by `awiki watchdog`. It
// observes BatchDir, debounces per-path writes, and delegates each
// settled path to runner.IngestSingle. On ingest failure the source is
// quarantined under FailedDir and the loop continues.
//
// Stdout records (slice 9 plan):
//
//	WATCHDOG|event=start|root=<batch>
//	WATCHDOG|event=catchup|count=<n>          (only when --catchup)
//	WATCHDOG|event=fsnotify-init-failed|err=. (fall-through to poll)
//	WATCHDOG|event=ingested|path=<rel>
//	WATCHDOG|event=fail|path=<rel>|err=<msg>
//	WATCHDOG|event=stop
//
// On context cancellation Watchdog emits the stop record and returns
// nil. SIGINT/SIGTERM handling is the dispatcher's responsibility — it
// converts signals to ctx cancellation.
func Watchdog(ctx context.Context, runner *WatchdogRunner, opts WatchdogOptions, stdout, stderr io.Writer) error {
	if runner == nil || runner.Runner == nil {
		return errors.New("watchdog: runner is nil")
	}
	ingestFn := runner.IngestSingle
	if ingestFn == nil {
		ingestFn = runner.Runner.IngestSingle
	}

	batch := opts.BatchDir
	if batch == "" {
		batch = runner.Runner.BatchDir()
	}
	failed := opts.FailedDir
	if failed == "" {
		failed = filepath.Join(batch, "_failed")
	}

	if err := os.MkdirAll(batch, 0o755); err != nil {
		return fmt.Errorf("watchdog: mkdir batch: %w", err)
	}
	if err := os.MkdirAll(failed, 0o755); err != nil {
		return fmt.Errorf("watchdog: mkdir failed: %w", err)
	}

	debounce := time.Duration(opts.DebounceMs) * time.Millisecond
	if debounce == 0 {
		debounce = 500 * time.Millisecond
	}

	fmt.Fprintf(stdout, "WATCHDOG|event=start|root=%s\n", relRoot(runner.Runner.RepoRoot, batch))

	// Local cancellation so the watcher / poller goroutine exits when
	// Watchdog returns (e.g. on --once after one cycle).
	loopCtx, loopCancel := context.WithCancel(ctx)
	defer loopCancel()

	events := make(chan string, 64)

	// Catchup: drain existing files first. Each goes through the same
	// event channel so debounce + dispatch are centralized.
	catchupCount := 0
	if opts.Catchup {
		_ = filepath.Walk(batch, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				// Skip the _failed/ subtree entirely.
				if path == failed || strings.HasPrefix(path+string(filepath.Separator), failed+string(filepath.Separator)) {
					return filepath.SkipDir
				}
				return nil
			}
			abs := path
			if a, err := filepath.Abs(abs); err == nil {
				abs = a
			}
			catchupCount++
			// Non-blocking enqueue is fine; the channel is sized 64 and
			// the consumer drains before blocking. If the catchup set is
			// larger than 64, fall back to a blocking send guarded by
			// loopCtx so a pre-cancelled run does not deadlock.
			select {
			case events <- abs:
			case <-loopCtx.Done():
				return errors.New("ctx cancelled")
			}
			return nil
		})
		fmt.Fprintf(stdout, "WATCHDOG|event=catchup|count=%d\n", catchupCount)
	}

	// Source the events channel: either fsnotify or polling.
	usePoll := opts.PollInterval > 0
	pollInterval := opts.PollInterval

	if !usePoll {
		// Try fsnotify. On init failure log and fall through to poll@2s.
		if runner.Runner.FSNotify == nil {
			fmt.Fprintf(stdout, "WATCHDOG|event=fsnotify-init-failed|err=adapter-nil\n")
			usePoll = true
			pollInterval = 2 * time.Second
		} else {
			// Probe the watcher in a goroutine. If Watch returns an
			// error before the consumer processes any event, treat it as
			// init failure and fall through. We do this by starting the
			// watch in a goroutine and waiting briefly on a sentinel
			// channel for the success/failure signal.
			watchErrCh := make(chan error, 1)
			go func() {
				watchErrCh <- runner.Runner.FSNotify.Watch(loopCtx, batch, events)
			}()
			// Give the watcher a brief moment to fail-fast on init. The
			// fake adapter used in tests returns its FailInit error
			// immediately, so a short wait suffices.
			select {
			case err := <-watchErrCh:
				if err != nil {
					fmt.Fprintf(stdout, "WATCHDOG|event=fsnotify-init-failed|err=%s\n", err.Error())
					usePoll = true
					pollInterval = 2 * time.Second
				} else {
					// Watcher exited cleanly (ctx cancel before any
					// event). Treat as a no-op, but still fall through
					// to poll so the loop has a source if Once is set.
					usePoll = true
					pollInterval = 2 * time.Second
				}
			case <-time.After(20 * time.Millisecond):
				// Watcher is alive; let it run.
			case <-loopCtx.Done():
				// Bail early; the deferred cancel will tear down.
			}
		}
	}

	if usePoll {
		go pollLoop(loopCtx, batch, failed, pollInterval, events)
	}

	// Per-path debounce: each path has a timer; on each new event the
	// timer is reset. When it fires, we dispatch ingest.
	var (
		mu     sync.Mutex
		timers = map[string]*time.Timer{}
	)

	// dispatch is invoked when a path settles. It runs in the timer's
	// goroutine. It must not block the event loop.
	dispatch := func(path string) {
		mu.Lock()
		delete(timers, path)
		mu.Unlock()
		// Skip if the path no longer exists (deleted between debounce
		// arming and firing).
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			return
		}
		// Skip files under _failed/.
		if isUnderFailed(path, failed) {
			return
		}

		var iStdout, iStderr strings.Builder
		ingestErr := ingestFn(loopCtx, path, &iStdout, &iStderr)
		rel := relRoot(runner.Runner.RepoRoot, path)
		if ingestErr == nil {
			fmt.Fprintf(stdout, "WATCHDOG|event=ingested|path=%s\n", rel)
			return
		}
		// Quarantine: move source to failed/.
		dst := filepath.Join(failed, filepath.Base(path))
		if mvErr := os.Rename(path, dst); mvErr != nil {
			// Fall through; we still emit fail so the user sees it.
			fmt.Fprintf(stdout, "WATCHDOG|event=fail|path=%s|err=%s\n", rel, sanitize(ingestErr.Error()))
			return
		}
		fmt.Fprintf(stdout, "WATCHDOG|event=fail|path=%s|err=%s\n", rel, sanitize(ingestErr.Error()))
	}

	// onceDone signals the main loop to exit after the first dispatched
	// event when Once is set. We close it from the dispatch wrapper.
	onceDone := make(chan struct{})
	var onceClose sync.Once
	wrappedDispatch := dispatch
	if opts.Once {
		wrappedDispatch = func(path string) {
			dispatch(path)
			onceClose.Do(func() { close(onceDone) })
		}
	}

	for {
		select {
		case <-ctx.Done():
			fmt.Fprintln(stdout, "WATCHDOG|event=stop")
			return nil
		case <-onceDone:
			fmt.Fprintln(stdout, "WATCHDOG|event=stop")
			return nil
		case path, ok := <-events:
			if !ok {
				fmt.Fprintln(stdout, "WATCHDOG|event=stop")
				return nil
			}
			// Skip events under _failed/ at enqueue time too — saves a
			// timer alloc on noisy directories.
			if isUnderFailed(path, failed) {
				continue
			}
			mu.Lock()
			if t, exists := timers[path]; exists {
				t.Reset(debounce)
			} else {
				p := path
				timers[p] = time.AfterFunc(debounce, func() {
					wrappedDispatch(p)
				})
			}
			mu.Unlock()
		}
	}
}

// pollLoop is the polling fallback. It scans batch every interval and
// emits any new (or modified) regular file path on the events channel.
// Files under _failed/ are skipped.
func pollLoop(ctx context.Context, batch, failed string, interval time.Duration, events chan<- string) {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	known := map[string]int64{}
	scan := func() {
		_ = filepath.Walk(batch, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				if path == failed || strings.HasPrefix(path+string(filepath.Separator), failed+string(filepath.Separator)) {
					return filepath.SkipDir
				}
				return nil
			}
			abs := path
			if a, err := filepath.Abs(abs); err == nil {
				abs = a
			}
			size := info.Size()
			modUnix := info.ModTime().UnixNano()
			// Combine size + mtime into a single comparable token.
			tok := modUnix ^ size
			if prev, ok := known[abs]; !ok || prev != tok {
				known[abs] = tok
				select {
				case events <- abs:
				case <-ctx.Done():
					return errors.New("ctx cancelled")
				}
			}
			return nil
		})
	}
	// First scan happens immediately so tests need not wait the full
	// interval to observe a synthesized file.
	scan()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			scan()
		}
	}
}

// isUnderFailed reports whether path lies under failed (the quarantine
// directory). Used both to skip enqueue and to short-circuit dispatch.
func isUnderFailed(path, failed string) bool {
	if failed == "" {
		return false
	}
	if path == failed {
		return true
	}
	return strings.HasPrefix(path+string(filepath.Separator), failed+string(filepath.Separator))
}

// relRoot returns p relative to root when possible, otherwise p
// unchanged. Watchdog records use repo-relative paths so byte-identical
// output is reachable across `pwd` differences in tests.
func relRoot(root, p string) string {
	if root == "" {
		return p
	}
	if rel, err := filepath.Rel(root, p); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return p
}

// sanitize collapses newlines/CRs to spaces so error messages stay on
// a single WATCHDOG record line.
func sanitize(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}
