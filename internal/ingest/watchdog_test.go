package ingest

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"awiki/internal/testutil"
)

// newWatchdogTestRoot prepares an empty repo-shaped directory for a
// watchdog test. Returns the repo root, batch dir, and failed dir.
func newWatchdogTestRoot(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	batch := filepath.Join(root, "raw", "inbox", "batch")
	failed := filepath.Join(batch, "_failed")
	if err := os.MkdirAll(batch, 0o755); err != nil {
		t.Fatalf("mkdir batch: %v", err)
	}
	if err := os.MkdirAll(failed, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	return root, batch, failed
}

// countingIngest builds an IngestSingleFn that counts invocations and
// optionally returns a fixed error. It is the test stand-in for
// (*Runner).IngestSingle when we want deterministic ingest behaviour
// without staging a full source path.
func countingIngest(err error) (IngestSingleFn, *atomic.Int32) {
	var n atomic.Int32
	fn := func(_ context.Context, _ string, _, _ io.Writer) error {
		n.Add(1)
		return err
	}
	return fn, &n
}

// waitFor polls fn every 5ms up to timeout. Returns true if fn returned
// true within the budget. Tests use this to wait for asynchronous
// debounce dispatch without sleeping a fixed wall-clock duration.
func waitFor(timeout time.Duration, fn func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return fn()
}

// TestWatchdogSingleEventDispatchesOneIngest exercises the happy path:
// one synthesized event triggers exactly one debounced ingest call.
func TestWatchdogSingleEventDispatchesOneIngest(t *testing.T) {
	root, batch, _ := newWatchdogTestRoot(t)

	src := filepath.Join(batch, "note.md")
	if err := os.WriteFile(src, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	fake := &testutil.FakeFSNotify{In: make(chan string, 4)}
	ingestFn, calls := countingIngest(nil)

	r := &Runner{RepoRoot: root, FSNotify: fake}
	wr := &WatchdogRunner{Runner: r, IngestSingle: ingestFn}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var stdout, stderr bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- Watchdog(ctx, wr, WatchdogOptions{
			Once:       true,
			DebounceMs: 30,
			BatchDir:   batch,
		}, &stdout, &stderr)
	}()

	// Push the path through the fake adapter.
	fake.In <- src

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Watchdog: %v", err)
		}
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatalf("Watchdog did not exit on --once; stdout=%q", stdout.String())
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("ingest calls = %d, want 1; stdout=%q", got, stdout.String())
	}
	if !strings.Contains(stdout.String(), "WATCHDOG|event=ingested|path=raw/inbox/batch/note.md") {
		t.Fatalf("stdout missing ingested record: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "WATCHDOG|event=stop") {
		t.Fatalf("stdout missing stop record: %q", stdout.String())
	}
}

// TestWatchdogRapidWritesCoalesce asserts that multiple events for the
// same path within the debounce window collapse into a single ingest
// call.
func TestWatchdogRapidWritesCoalesce(t *testing.T) {
	root, batch, _ := newWatchdogTestRoot(t)

	src := filepath.Join(batch, "rapid.md")
	if err := os.WriteFile(src, []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	fake := &testutil.FakeFSNotify{In: make(chan string, 16)}
	ingestFn, calls := countingIngest(nil)
	r := &Runner{RepoRoot: root, FSNotify: fake}
	wr := &WatchdogRunner{Runner: r, IngestSingle: ingestFn}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var stdout, stderr bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- Watchdog(ctx, wr, WatchdogOptions{
			Once:       true,
			DebounceMs: 80,
			BatchDir:   batch,
		}, &stdout, &stderr)
	}()

	// Burst: many events for the same path, all within the debounce
	// window. The watchdog must coalesce them into one ingest call.
	for i := 0; i < 10; i++ {
		fake.In <- src
		time.Sleep(5 * time.Millisecond)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Watchdog: %v", err)
		}
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatalf("Watchdog did not exit on --once; stdout=%q", stdout.String())
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("ingest calls = %d, want 1 (debounce coalesce)", got)
	}
}

// TestWatchdogIngestFailureQuarantines asserts that a failing ingest
// moves the source under failed/ and emits the fail record.
func TestWatchdogIngestFailureQuarantines(t *testing.T) {
	root, batch, failed := newWatchdogTestRoot(t)

	src := filepath.Join(batch, "broken.md")
	if err := os.WriteFile(src, []byte("nope\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	fake := &testutil.FakeFSNotify{In: make(chan string, 4)}
	ingestFn, _ := countingIngest(errors.New("synthetic ingest failure"))
	r := &Runner{RepoRoot: root, FSNotify: fake}
	wr := &WatchdogRunner{Runner: r, IngestSingle: ingestFn}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var stdout, stderr bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- Watchdog(ctx, wr, WatchdogOptions{
			Once:       true,
			DebounceMs: 30,
			BatchDir:   batch,
		}, &stdout, &stderr)
	}()

	fake.In <- src

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Watchdog: %v", err)
		}
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatalf("Watchdog did not exit on --once; stdout=%q", stdout.String())
	}

	// Source moved to failed/.
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("source still present in batch: stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(failed, "broken.md")); err != nil {
		t.Fatalf("source not quarantined: %v", err)
	}
	if !strings.Contains(stdout.String(), "WATCHDOG|event=fail|path=raw/inbox/batch/broken.md|err=synthetic ingest failure") {
		t.Fatalf("stdout missing fail record: %q", stdout.String())
	}
}

// TestWatchdogCatchupDrainsExistingFiles asserts --catchup walks the
// batch dir at startup and enqueues each existing file before entering
// the watch loop. Files under _failed/ are skipped.
func TestWatchdogCatchupDrainsExistingFiles(t *testing.T) {
	root, batch, failed := newWatchdogTestRoot(t)

	// Two real sources + one quarantined file (must be skipped).
	for _, name := range []string{"a.md", "b.md"} {
		if err := os.WriteFile(filepath.Join(batch, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(failed, "old.md"), []byte("quarantined"), 0o644); err != nil {
		t.Fatal(err)
	}

	fake := &testutil.FakeFSNotify{In: make(chan string, 4)}
	ingestFn, calls := countingIngest(nil)
	r := &Runner{RepoRoot: root, FSNotify: fake}
	wr := &WatchdogRunner{Runner: r, IngestSingle: ingestFn}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var stdout, stderr bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- Watchdog(ctx, wr, WatchdogOptions{
			Catchup:    true,
			DebounceMs: 30,
			BatchDir:   batch,
		}, &stdout, &stderr)
	}()

	// Wait for the two catchup files to be ingested, then cancel.
	if !waitFor(2*time.Second, func() bool { return calls.Load() == 2 }) {
		cancel()
		<-done
		t.Fatalf("ingest calls = %d, want 2; stdout=%q", calls.Load(), stdout.String())
	}
	cancel()
	<-done

	if !strings.Contains(stdout.String(), "WATCHDOG|event=catchup|count=2") {
		t.Fatalf("stdout missing catchup record (count=2): %q", stdout.String())
	}
}

// TestWatchdogOnceExitsAfterOneEvent asserts --once drives a clean exit
// after a single dispatched ingest, even if more events would follow.
func TestWatchdogOnceExitsAfterOneEvent(t *testing.T) {
	root, batch, _ := newWatchdogTestRoot(t)

	src1 := filepath.Join(batch, "first.md")
	src2 := filepath.Join(batch, "second.md")
	for _, p := range []string{src1, src2} {
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	fake := &testutil.FakeFSNotify{In: make(chan string, 4)}
	ingestFn, calls := countingIngest(nil)
	r := &Runner{RepoRoot: root, FSNotify: fake}
	wr := &WatchdogRunner{Runner: r, IngestSingle: ingestFn}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var stdout, stderr bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- Watchdog(ctx, wr, WatchdogOptions{
			Once:       true,
			DebounceMs: 30,
			BatchDir:   batch,
		}, &stdout, &stderr)
	}()

	fake.In <- src1
	// Push a second event a moment later — by the time the dispatch
	// happens, --once should have fired.
	go func() {
		time.Sleep(80 * time.Millisecond)
		fake.In <- src2
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Watchdog: %v", err)
		}
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatalf("Watchdog did not exit on --once; stdout=%q", stdout.String())
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("ingest calls = %d, want exactly 1 (--once)", got)
	}
}

// TestWatchdogPollIntervalPicksUpFiles asserts that PollInterval > 0
// drives the polling backend (no fsnotify), and that files synthesized
// directly on disk are observed.
func TestWatchdogPollIntervalPicksUpFiles(t *testing.T) {
	root, batch, _ := newWatchdogTestRoot(t)

	ingestFn, calls := countingIngest(nil)
	// FSNotify should NOT be invoked under PollInterval > 0.
	fake := &testutil.FakeFSNotify{FailInit: errors.New("must not be called")}
	r := &Runner{RepoRoot: root, FSNotify: fake}
	wr := &WatchdogRunner{Runner: r, IngestSingle: ingestFn}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var stdout, stderr bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = Watchdog(ctx, wr, WatchdogOptions{
			PollInterval: 50 * time.Millisecond,
			DebounceMs:   30,
			BatchDir:     batch,
		}, &stdout, &stderr)
	}()

	// Synthesize a file after a short delay so the first poll picks it
	// up. The poll loop scans immediately on entry plus every interval.
	time.Sleep(20 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(batch, "polled.md"), []byte("p\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !waitFor(2*time.Second, func() bool { return calls.Load() >= 1 }) {
		cancel()
		wg.Wait()
		t.Fatalf("poll did not pick up file; stdout=%q", stdout.String())
	}

	cancel()
	wg.Wait()

	if got := calls.Load(); got < 1 {
		t.Fatalf("ingest calls = %d, want >= 1", got)
	}
	// fsnotify init record should not appear: PollInterval forces poll
	// without ever consulting the fsnotify adapter.
	if strings.Contains(stdout.String(), "fsnotify-init-failed") {
		t.Fatalf("stdout should not announce fsnotify init failure under PollInterval: %q", stdout.String())
	}
}

// TestWatchdogFSNotifyInitFailFallsThroughToPoll asserts that a fake
// adapter returning an init error causes the watchdog to log the
// fall-through and drive its events from the polling backend instead.
func TestWatchdogFSNotifyInitFailFallsThroughToPoll(t *testing.T) {
	root, batch, _ := newWatchdogTestRoot(t)

	fake := &testutil.FakeFSNotify{FailInit: testutil.ErrFSNotifyInit}
	ingestFn, calls := countingIngest(nil)
	r := &Runner{RepoRoot: root, FSNotify: fake}
	wr := &WatchdogRunner{Runner: r, IngestSingle: ingestFn}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var stdout, stderr bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = Watchdog(ctx, wr, WatchdogOptions{
			DebounceMs: 30,
			BatchDir:   batch,
		}, &stdout, &stderr)
	}()

	// Wait for the fall-through record before staging any file. That
	// pins the order: init failure -> poll loop active.
	if !waitFor(1*time.Second, func() bool {
		return strings.Contains(stdout.String(), "WATCHDOG|event=fsnotify-init-failed|err=fake fsnotify: init failed")
	}) {
		cancel()
		wg.Wait()
		t.Fatalf("fall-through record not observed; stdout=%q", stdout.String())
	}

	// Override the poll interval is 2s default; just synthesize the
	// file and rely on the immediate scan on tick. The poll loop scans
	// on entry, so we should see the file within ~2s.
	if err := os.WriteFile(filepath.Join(batch, "fallback.md"), []byte("f\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !waitFor(4*time.Second, func() bool { return calls.Load() >= 1 }) {
		cancel()
		wg.Wait()
		t.Fatalf("fallback poll did not pick up file; stdout=%q", stdout.String())
	}

	cancel()
	wg.Wait()
}

// TestWatchdogContextCancelEmitsStop asserts SIGINT-equivalent (ctx
// cancel) drives a graceful exit with the stop record on stdout.
func TestWatchdogContextCancelEmitsStop(t *testing.T) {
	root, batch, _ := newWatchdogTestRoot(t)

	fake := &testutil.FakeFSNotify{In: make(chan string)}
	ingestFn, _ := countingIngest(nil)
	r := &Runner{RepoRoot: root, FSNotify: fake}
	wr := &WatchdogRunner{Runner: r, IngestSingle: ingestFn}

	ctx, cancel := context.WithCancel(context.Background())

	var stdout, stderr bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- Watchdog(ctx, wr, WatchdogOptions{
			DebounceMs: 30,
			BatchDir:   batch,
		}, &stdout, &stderr)
	}()

	// Give the watcher a moment to install, then cancel.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Watchdog: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Watchdog did not exit on ctx cancel; stdout=%q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "WATCHDOG|event=start|root=raw/inbox/batch") {
		t.Fatalf("stdout missing start record: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "WATCHDOG|event=stop") {
		t.Fatalf("stdout missing stop record: %q", stdout.String())
	}
}
