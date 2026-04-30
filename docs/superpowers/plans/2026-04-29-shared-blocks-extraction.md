# Shared building blocks extraction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Lift helpers that already live inside `internal/lint/` (atomic
write, advisory lock, record formatters, region parsers, action grammar)
into shared packages under `internal/{fsutil,emit,region,action,config}`,
and add typed adapter interface stubs under `internal/adapters/`. No
behavior change. This is the prep slice for the Go-port roadmap
(`docs/superpowers/specs/2026-04-29-go-port-roadmap-design.md`); the
synth domain port and Slice 0 lint cleanup land in their own plans.

**Architecture:** Pure refactor. Each task moves one helper, updates the
single existing consumer (`internal/lint`), keeps tests green, commits.
The new packages are imported by zero non-lint code today; the synth
spec is the first downstream consumer.

**Tech Stack:** Go 1.x (matches `go.mod`), `go test ./...`, `bats
tests/`. No new third-party deps in this slice.

---

## File structure

Created:
- `internal/fsutil/atomic.go` — atomic write helper.
- `internal/fsutil/atomic_test.go`.
- `internal/fsutil/lock.go` — advisory file lock.
- `internal/fsutil/lock_test.go`.
- `internal/emit/records.go` — typed record formatters.
- `internal/emit/records_test.go`.
- `internal/region/generated.go` — synth-flavor BEGIN/END GENERATED.
- `internal/region/generated_test.go`.
- `internal/region/managed.go` — managed-region.sh-flavor `<kind>:<id>`.
- `internal/region/managed_test.go`.
- `internal/action/grammar.go` — action-line parser (lifted from
  `internal/lint/task/parser.go`).
- `internal/action/grammar_test.go`.
- `internal/config/awiki.go` — `.awiki/config` loader.
- `internal/config/awiki_test.go`.
- `internal/adapters/interfaces.go` — typed interface stubs for hugo,
  qmd, duckdb, vlconvert, pdftotext, whisper, git, gpg, fsnotify.
- `internal/adapters/interfaces_test.go` — interface-shape pinning
  (compile-time assertions only).

Modified:
- `internal/lint/fix.go` — drop `atomicWriteFile`; import from
  `awiki/internal/fsutil`.
- `internal/lint/diagnostic.go` — record formatting delegates to
  `awiki/internal/emit`.
- `internal/lint/synth/region.go` — re-exports from
  `awiki/internal/region` (or imports + thin shim if call sites resist
  rename).
- `internal/lint/task/parser.go` — re-exports from
  `awiki/internal/action`.

Each task moves one boundary, one PR-equivalent commit. Lint stays
fully passing throughout.

---

### Task 1: Extract `internal/fsutil` (atomic write)

**Files:**
- Create: `internal/fsutil/atomic.go`
- Create: `internal/fsutil/atomic_test.go`
- Modify: `internal/lint/fix.go`

- [ ] **Step 1: Read the current implementation**

Read `internal/lint/fix.go`. Locate `atomicWriteFile` and
`closeTempFile` (lines ~80-115). Note the signature:

```go
func atomicWriteFile(path string, data []byte) error
func closeTempFile(file *os.File, err error) error
```

- [ ] **Step 2: Write a failing test for `fsutil.AtomicWrite`**

Create `internal/fsutil/atomic_test.go`:

```go
package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWritePreservesPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "page.md")
	if err := os.WriteFile(path, []byte("old\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWrite(path, []byte("new\n")); err != nil {
		t.Fatalf("AtomicWrite: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new\n" {
		t.Fatalf("contents: got %q want %q", got, "new\n")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o640 {
		t.Fatalf("perm: got %o want 0640", perm)
	}
}

func TestAtomicWriteLeavesNoTempOnSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "page.md")
	if err := os.WriteFile(path, []byte("seed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWrite(path, []byte("body")); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file in dir, got %d", len(entries))
	}
}

func TestAtomicWriteRequiresExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.md")
	if err := AtomicWrite(path, []byte("x")); err == nil {
		t.Fatalf("expected error when target file does not exist")
	}
}
```

- [ ] **Step 3: Run the test to verify it fails**

```bash
go test ./internal/fsutil/...
```

Expected: build error (`AtomicWrite` undefined).

- [ ] **Step 4: Write `internal/fsutil/atomic.go`**

```go
// Package fsutil provides shared filesystem helpers (atomic write,
// advisory locking) used across awiki domain packages.
package fsutil

import (
	"os"
	"path/filepath"
)

// AtomicWrite replaces the file at path with data, preserving the
// existing file's mode bits. Implemented as write-to-temp + rename so
// readers never see a partial file. The target file must already exist;
// callers create new files with os.WriteFile.
func AtomicWrite(path string, data []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	keepTemp := true
	defer func() {
		if keepTemp {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		return closeTempFile(tmp, err)
	}
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		return closeTempFile(tmp, err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	keepTemp = false
	return nil
}

func closeTempFile(file *os.File, err error) error {
	if closeErr := file.Close(); closeErr != nil && err == nil {
		return closeErr
	}
	return err
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/fsutil/...
```

Expected: PASS.

- [ ] **Step 6: Update `internal/lint/fix.go` to use fsutil**

In `internal/lint/fix.go`:

1. Add `"awiki/internal/fsutil"` to the imports.
2. Replace the call site `if err := atomicWriteFile(path, []byte(...))`
   with `if err := fsutil.AtomicWrite(path, []byte(...))`.
3. Delete the local `atomicWriteFile` and `closeTempFile` functions.

- [ ] **Step 7: Run lint tests to verify no regression**

```bash
go test ./...
```

Expected: PASS (all lint tests still pass, no `atomicWriteFile`
unused-symbol error).

- [ ] **Step 8: Commit**

```bash
git add internal/fsutil/atomic.go internal/fsutil/atomic_test.go \
        internal/lint/fix.go
git commit -m "refactor: extract atomic write into internal/fsutil"
```

---

### Task 2: Extend `internal/fsutil` (advisory lock)

**Files:**
- Create: `internal/fsutil/lock.go`
- Create: `internal/fsutil/lock_test.go`

- [ ] **Step 1: Read the bash original**

Read `scripts/lib/lock.sh`. Note: two timeouts (user=30s, deferred=180s),
exclusive vs shared modes, contention exits 7. Lock file lives at
`<repo>/.awiki/lock`. Implementation uses `flock(2)` via the `flock`
binary; Go port uses `golang.org/x/sys/unix.Flock`.

- [ ] **Step 2: Write the failing test**

Create `internal/fsutil/lock_test.go`:

```go
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
```

- [ ] **Step 3: Run the test, verify it fails**

```bash
go test ./internal/fsutil/...
```

Expected: build error (`WithExclusiveLock`, `WithSharedLock`,
`ContentionError` undefined).

- [ ] **Step 4: Implement `internal/fsutil/lock.go`**

```go
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
	defer unix.Flock(int(f.Fd()), unix.LOCK_UN)
	return body()
}
```

- [ ] **Step 5: Add the dep**

```bash
go get golang.org/x/sys/unix
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
go test ./internal/fsutil/...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/fsutil/lock.go internal/fsutil/lock_test.go go.mod go.sum
git commit -m "feat: add internal/fsutil advisory lock"
```

---

### Task 3: Extract `internal/emit` (record formatters)

**Files:**
- Create: `internal/emit/records.go`
- Create: `internal/emit/records_test.go`
- Modify: `internal/lint/diagnostic.go`

- [ ] **Step 1: Read existing record formats**

Read `internal/lint/diagnostic.go`. The two formats:

- `LINT|<level>|<file>|<message>` (no code) or
  `LINT|<level>|<file>|<code>|<message>` (with code).
- `FIX|<file>|<message>`.
- Summary line: `LINT-SUMMARY|errors=N|warnings=N|info=N`.

- [ ] **Step 2: Write the failing test for `emit.Lint`/`emit.Fix`**

Create `internal/emit/records_test.go`:

```go
package emit

import "testing"

func TestLintRecordWithoutCode(t *testing.T) {
	got := Lint("ERROR", "content/entities/foo.md", "", "broken wikilink: [[bar]]")
	want := "LINT|ERROR|content/entities/foo.md|broken wikilink: [[bar]]"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestLintRecordWithCode(t *testing.T) {
	got := Lint("ERROR", "content/datasets/bad.md", "D1", "missing storage")
	want := "LINT|ERROR|content/datasets/bad.md|D1|missing storage"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFixRecord(t *testing.T) {
	got := Fix("content/entities/foo.md", "added last_updated: 2026-04-29")
	want := "FIX|content/entities/foo.md|added last_updated: 2026-04-29"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestLintSummary(t *testing.T) {
	got := LintSummary(2, 1, 0)
	want := "LINT-SUMMARY|errors=2|warnings=1|info=0"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestAgentPromptRecord(t *testing.T) {
	got := AgentPrompt("ingest", []string{"path=raw/inbox/foo.md", "kind=entity"})
	want := "AGENT-PROMPT|ingest|path=raw/inbox/foo.md|kind=entity"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestIngestRecord(t *testing.T) {
	got := Ingest("xlsx", "raw/inbox/batch/sales.xlsx", "ok")
	want := "INGEST|xlsx|raw/inbox/batch/sales.xlsx|ok"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestReviewRecord(t *testing.T) {
	got := Review("week", "2026-W17", []string{"open=12", "done=4"})
	want := "REVIEW|week|2026-W17|open=12|done=4"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRecurRecord(t *testing.T) {
	got := Recur("content/inbox.md", "abc123", "due=2026-05-06")
	want := "RECUR|content/inbox.md|abc123|due=2026-05-06"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
```

- [ ] **Step 3: Run the test, verify it fails**

```bash
go test ./internal/emit/...
```

Expected: build error (package `emit` undefined).

- [ ] **Step 4: Implement `internal/emit/records.go`**

```go
// Package emit owns the user-visible record formats awiki commands
// write to stdout. Domain code never assembles record strings by hand;
// it calls the typed formatters here.
package emit

import (
	"fmt"
	"strings"
)

// Lint formats a LINT| record. code may be empty.
func Lint(level, file, code, message string) string {
	if code == "" {
		return fmt.Sprintf("LINT|%s|%s|%s", level, file, message)
	}
	return fmt.Sprintf("LINT|%s|%s|%s|%s", level, file, code, message)
}

// Fix formats a FIX| record.
func Fix(file, message string) string {
	return fmt.Sprintf("FIX|%s|%s", file, message)
}

// LintSummary formats the trailing summary line for `awiki lint`.
func LintSummary(errors, warnings, infos int) string {
	return fmt.Sprintf("LINT-SUMMARY|errors=%d|warnings=%d|info=%d",
		errors, warnings, infos)
}

// AgentPrompt formats an AGENT-PROMPT| record. fields are emitted in
// the order given (the bash callers preserve insertion order).
func AgentPrompt(verb string, fields []string) string {
	if len(fields) == 0 {
		return "AGENT-PROMPT|" + verb
	}
	return "AGENT-PROMPT|" + verb + "|" + strings.Join(fields, "|")
}

// Ingest formats an INGEST| record.
func Ingest(format, source, status string) string {
	return fmt.Sprintf("INGEST|%s|%s|%s", format, source, status)
}

// Review formats a REVIEW| record. extra fields are joined with '|'.
func Review(kind, period string, fields []string) string {
	head := fmt.Sprintf("REVIEW|%s|%s", kind, period)
	if len(fields) == 0 {
		return head
	}
	return head + "|" + strings.Join(fields, "|")
}

// Recur formats a RECUR| record produced by action-recur.
func Recur(file, blockID string, fields ...string) string {
	head := fmt.Sprintf("RECUR|%s|%s", file, blockID)
	if len(fields) == 0 {
		return head
	}
	return head + "|" + strings.Join(fields, "|")
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/emit/...
```

Expected: PASS.

- [ ] **Step 6: Wire `emit` through `internal/lint/diagnostic.go`**

In `internal/lint/diagnostic.go`:

1. Add `"awiki/internal/emit"` to imports.
2. Replace the body of `Diagnostic.Record()` with
   `return emit.Lint(string(d.Level), d.File, d.Code, d.Message)`.
3. Replace `FixRecord.Record()` body with
   `return emit.Fix(r.File, r.Message)`.
4. Replace `Collector.Summary()` body with
   `errors, warnings, infos := c.Counts(); return emit.LintSummary(errors, warnings, infos)`.

Leave `Collector` and the level constants in `internal/lint` (they are
lint-specific). Only the string-shape lives in `emit`.

- [ ] **Step 7: Run all tests to verify no regression**

```bash
go test ./...
```

Expected: PASS. Existing `diagnostic_test.go` cases still pass byte-for-
byte because the format is unchanged.

- [ ] **Step 8: Commit**

```bash
git add internal/emit/records.go internal/emit/records_test.go \
        internal/lint/diagnostic.go
git commit -m "refactor: route lint records through internal/emit"
```

---

### Task 4: Extract `internal/region` (synth-flavor)

**Files:**
- Create: `internal/region/generated.go`
- Create: `internal/region/generated_test.go`
- Modify: `internal/lint/synth/region.go`

- [ ] **Step 1: Read the existing implementation**

Read `internal/lint/synth/region.go`. Note: parses
`<!-- BEGIN GENERATED ... -->` and `<!-- END GENERATED -->` with
diagnostics for missing/duplicate/out-of-order markers. Returns
`GeneratedRegion` plus `[]RegionDiagnostic`.

- [ ] **Step 2: Write the failing test (one well-formed + one missing)**

Create `internal/region/generated_test.go`:

```go
package region

import "testing"

func TestParseGeneratedHappyPath(t *testing.T) {
	text := "intro\n<!-- BEGIN GENERATED v1 -->\nbody\n<!-- END GENERATED -->\nfooter\n"
	got, diags := ParseGenerated(text)
	if len(diags) != 0 {
		t.Fatalf("diagnostics: %+v", diags)
	}
	if got.Body != "body\n" {
		t.Fatalf("body: %q", got.Body)
	}
	if got.BeginLine != "<!-- BEGIN GENERATED v1 -->" {
		t.Fatalf("begin line: %q", got.BeginLine)
	}
}

func TestParseGeneratedMissingBegin(t *testing.T) {
	_, diags := ParseGenerated("no markers here\n")
	if len(diags) == 0 {
		t.Fatalf("expected diagnostics for missing markers")
	}
}

func TestParseGeneratedDuplicateBegin(t *testing.T) {
	text := "<!-- BEGIN GENERATED a -->\n<!-- BEGIN GENERATED b -->\n<!-- END GENERATED -->\n"
	_, diags := ParseGenerated(text)
	if len(diags) == 0 {
		t.Fatalf("expected diagnostics for duplicate BEGIN")
	}
}
```

- [ ] **Step 3: Run the test, verify it fails**

```bash
go test ./internal/region/...
```

Expected: build error (`ParseGenerated` undefined).

- [ ] **Step 4: Move the implementation**

Create `internal/region/generated.go` with the body of
`internal/lint/synth/region.go`, renamed:

```go
// Package region parses awiki's two managed-region marker styles.
package region

import "strings"

const (
	beginMarker = "<!-- BEGIN GENERATED"
	endMarker   = "<!-- END GENERATED -->"
)

// Generated is the synth-flavor region: a single
// `<!-- BEGIN GENERATED ... -->` ... `<!-- END GENERATED -->` block.
type Generated struct {
	BeginOffset int
	EndOffset   int
	Body        string
	BeginLine   string
}

// Diagnostic is a marker-level lint emitted by ParseGenerated. The
// caller maps it to a domain Diagnostic (synth lint uses code S1).
type Diagnostic struct {
	Code    string
	Message string
}

// ParseGenerated extracts the single generated region from text.
func ParseGenerated(text string) (Generated, []Diagnostic) {
	beginOffsets := markerOffsets(text, beginMarker)
	endOffsets := markerOffsets(text, endMarker)
	var diagnostics []Diagnostic
	if len(beginOffsets) != 1 {
		diagnostics = append(diagnostics, Diagnostic{
			Code:    "S1",
			Message: "expected exactly one BEGIN GENERATED marker",
		})
	}
	if len(endOffsets) != 1 {
		diagnostics = append(diagnostics, Diagnostic{
			Code:    "S1",
			Message: "expected exactly one END GENERATED marker",
		})
	}
	if len(beginOffsets) != 1 || len(endOffsets) != 1 {
		return Generated{BeginOffset: -1, EndOffset: -1}, diagnostics
	}

	beginOffset := beginOffsets[0]
	endOffset := endOffsets[0]
	if beginOffset > endOffset {
		diagnostics = append(diagnostics, Diagnostic{
			Code:    "S1",
			Message: "BEGIN GENERATED marker must appear before END GENERATED marker",
		})
		return Generated{BeginOffset: beginOffset, EndOffset: endOffset}, diagnostics
	}

	beginLineEnd := strings.IndexByte(text[beginOffset:], '\n')
	if beginLineEnd < 0 {
		beginLineEnd = len(text) - beginOffset
	}
	beginLineEnd += beginOffset
	bodyStart := beginLineEnd
	if bodyStart < len(text) && text[bodyStart] == '\n' {
		bodyStart++
	}
	return Generated{
		BeginOffset: beginOffset,
		EndOffset:   endOffset,
		Body:        text[bodyStart:endOffset],
		BeginLine:   text[beginOffset:beginLineEnd],
	}, diagnostics
}

func markerOffsets(text, marker string) []int {
	var offsets []int
	searchFrom := 0
	for {
		next := strings.Index(text[searchFrom:], marker)
		if next < 0 {
			return offsets
		}
		offset := searchFrom + next
		offsets = append(offsets, offset)
		searchFrom = offset + len(marker)
	}
}
```

- [ ] **Step 5: Replace `internal/lint/synth/region.go` with a thin re-export**

Update `internal/lint/synth/region.go` to delegate:

```go
package synth

import "awiki/internal/region"

type GeneratedRegion = region.Generated
type RegionDiagnostic = region.Diagnostic

func ParseGeneratedRegion(text string) (GeneratedRegion, []RegionDiagnostic) {
	return region.ParseGenerated(text)
}
```

This keeps every existing call site in `internal/lint/synth/` compiling
unchanged.

- [ ] **Step 6: Run tests to verify everything passes**

```bash
go test ./...
```

Expected: PASS. The synth `region_test.go` continues to exercise the
same logic via the type alias.

- [ ] **Step 7: Commit**

```bash
git add internal/region/generated.go internal/region/generated_test.go \
        internal/lint/synth/region.go
git commit -m "refactor: extract synth region parser into internal/region"
```

---

### Task 5: Extend `internal/region` (managed-region flavor)

**Files:**
- Create: `internal/region/managed.go`
- Create: `internal/region/managed_test.go`

- [ ] **Step 1: Read the bash original**

Read `scripts/lib/managed-region.sh`. Marker style:

```
<!-- BEGIN <kind>:<id> -->
<body...>
<!-- END <kind>:<id> -->
```

Two operations: `replace` (insert if missing, replace body if present)
and `extract` (return body).

- [ ] **Step 2: Write the failing test**

Create `internal/region/managed_test.go`:

```go
package region

import "testing"

func TestManagedReplaceInsertsWhenAbsent(t *testing.T) {
	in := "intro\n"
	got, err := ManagedReplace(in, "agenda", "today", "line A\nline B\n")
	if err != nil {
		t.Fatal(err)
	}
	want := "intro\n\n<!-- BEGIN agenda:today -->\nline A\nline B\n<!-- END agenda:today -->\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestManagedReplaceUpdatesWhenPresent(t *testing.T) {
	in := "head\n<!-- BEGIN agenda:today -->\nstale\n<!-- END agenda:today -->\ntail\n"
	got, err := ManagedReplace(in, "agenda", "today", "fresh\n")
	if err != nil {
		t.Fatal(err)
	}
	want := "head\n<!-- BEGIN agenda:today -->\nfresh\n<!-- END agenda:today -->\ntail\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestManagedReplaceIsIdempotent(t *testing.T) {
	in := "head\n<!-- BEGIN agenda:today -->\nbody\n<!-- END agenda:today -->\n"
	once, err := ManagedReplace(in, "agenda", "today", "body\n")
	if err != nil {
		t.Fatal(err)
	}
	twice, err := ManagedReplace(once, "agenda", "today", "body\n")
	if err != nil {
		t.Fatal(err)
	}
	if once != twice {
		t.Fatalf("not idempotent:\nonce: %q\ntwice: %q", once, twice)
	}
}

func TestManagedExtract(t *testing.T) {
	in := "x\n<!-- BEGIN k:i -->\nhello\n<!-- END k:i -->\ny\n"
	got, ok := ManagedExtract(in, "k", "i")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got != "hello" {
		t.Fatalf("got %q want %q", got, "hello")
	}
}

func TestManagedExtractMissing(t *testing.T) {
	if _, ok := ManagedExtract("nothing here\n", "k", "i"); ok {
		t.Fatal("expected ok=false on missing region")
	}
}
```

- [ ] **Step 3: Run the test, verify it fails**

```bash
go test ./internal/region/...
```

Expected: build error (`ManagedReplace`, `ManagedExtract` undefined).

- [ ] **Step 4: Implement `internal/region/managed.go`**

```go
package region

import (
	"regexp"
	"strings"
)

// ManagedReplace returns text with the named managed region's body
// replaced by body. If the region is absent, ManagedReplace appends a
// new region to the end. The marker style is
// `<!-- BEGIN <kind>:<id> -->` / `<!-- END <kind>:<id> -->`.
//
// body must end with a newline; ManagedReplace adds one if missing.
func ManagedReplace(text, kind, id, body string) (string, error) {
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	begin := "<!-- BEGIN " + kind + ":" + id + " -->"
	end := "<!-- END " + kind + ":" + id + " -->"
	newBlock := begin + "\n" + body + end
	pat, err := managedPattern(kind, id)
	if err != nil {
		return "", err
	}
	if pat.MatchString(text) {
		return pat.ReplaceAllString(text, newBlock), nil
	}
	sep := ""
	if !strings.HasSuffix(text, "\n") {
		sep = "\n"
	}
	return text + sep + "\n" + newBlock + "\n", nil
}

// ManagedExtract returns the body inside the named region. ok=false
// when the region is absent.
func ManagedExtract(text, kind, id string) (string, bool) {
	pat, err := managedExtractPattern(kind, id)
	if err != nil {
		return "", false
	}
	m := pat.FindStringSubmatch(text)
	if m == nil {
		return "", false
	}
	return m[1], true
}

func managedPattern(kind, id string) (*regexp.Regexp, error) {
	return regexp.Compile(`(?s)<!-- BEGIN ` + regexp.QuoteMeta(kind) +
		`:` + regexp.QuoteMeta(id) + ` -->\n.*?\n<!-- END ` +
		regexp.QuoteMeta(kind) + `:` + regexp.QuoteMeta(id) + ` -->`)
}

func managedExtractPattern(kind, id string) (*regexp.Regexp, error) {
	return regexp.Compile(`(?s)<!-- BEGIN ` + regexp.QuoteMeta(kind) +
		`:` + regexp.QuoteMeta(id) + ` -->\n(.*?)\n<!-- END ` +
		regexp.QuoteMeta(kind) + `:` + regexp.QuoteMeta(id) + ` -->`)
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/region/...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/region/managed.go internal/region/managed_test.go
git commit -m "feat: add managed-region helper to internal/region"
```

---

### Task 6: Extract `internal/action` (action grammar)

**Files:**
- Create: `internal/action/grammar.go`
- Create: `internal/action/grammar_test.go`
- Modify: `internal/lint/task/parser.go`

- [ ] **Step 1: Read the existing parser**

Read `internal/lint/task/parser.go` and `internal/lint/task/types.go`.
Identify exported entry point `ParseActionLine(line string) (Action,
bool)` and the `Action` struct it returns. The parser is the Go port
of `scripts/lib/action-grammar.sh` and is consumed by `lint/task` rules
T1-T15.

- [ ] **Step 2: Write the failing test**

Create `internal/action/grammar_test.go`:

```go
package action

import "testing"

func TestParseSimpleOpen(t *testing.T) {
	a, ok := ParseLine("- [ ] write spec @home due:2026-05-01 ^abc123")
	if !ok {
		t.Fatal("expected match")
	}
	if a.Status != " " {
		t.Fatalf("status: %q", a.Status)
	}
	if a.Text != "write spec" {
		t.Fatalf("text: %q", a.Text)
	}
	if a.Context != "@home" {
		t.Fatalf("context: %q", a.Context)
	}
	if a.TailKeys["due"] != "2026-05-01" {
		t.Fatalf("due: %q", a.TailKeys["due"])
	}
	if a.ID != "abc123" {
		t.Fatalf("id: %q", a.ID)
	}
}

func TestParseRejectsDoubleCheckbox(t *testing.T) {
	if _, ok := ParseLine("- [ ] [ ] doubled"); ok {
		t.Fatal("expected reject")
	}
}

func TestParseBadDate(t *testing.T) {
	a, ok := ParseLine("- [ ] task due:not-a-date ^abc123")
	if !ok {
		t.Fatal("expected match")
	}
	if a.BadDate == "" {
		t.Fatal("expected BadDate to be set")
	}
}
```

- [ ] **Step 3: Run the test, verify it fails**

```bash
go test ./internal/action/...
```

Expected: build error (`ParseLine`, `Action` undefined).

- [ ] **Step 4: Move the parser**

Create `internal/action/grammar.go` by moving the bodies of
`internal/lint/task/parser.go` and the relevant fields of
`internal/lint/task/types.go` into the new package, renaming
`ParseActionLine` to `ParseLine` and `task.Action` to `action.Action`.
Also move helpers (`isDateKey`, `validDate`, the regex vars, the
`allowedTailKeys` map) into `internal/action/grammar.go`.

The exported surface:

```go
package action

type Action struct {
	Raw       string
	Status    string
	Text      string
	Context   string
	ID        string
	TailKeys  map[string]string
	Due       string
	Defer     string
	Wait      string
	Since     string
	Every     string
	Done      string
	Priority  string
	Estimate  string
	BadStatus string
	BadKey    string
	BadID     string
	BadDate   string
	HasBadID  bool
}

func ParseLine(line string) (Action, bool) { /* ... */ }
```

- [ ] **Step 5: Replace `internal/lint/task/parser.go` with re-exports**

Replace the file body with:

```go
package task

import "awiki/internal/action"

type Action = action.Action

func ParseActionLine(line string) (Action, bool) {
	return action.ParseLine(line)
}
```

Remove any `Action` field declarations from `internal/lint/task/types.go`
that now live in `internal/action`. Leave types that are task-lint-only
(e.g. rule-specific intermediate state) where they are.

- [ ] **Step 6: Run all tests to verify no regression**

```bash
go test ./...
```

Expected: PASS. Task lint rule tests continue to exercise the parser
via the alias.

- [ ] **Step 7: Commit**

```bash
git add internal/action/grammar.go internal/action/grammar_test.go \
        internal/lint/task/parser.go internal/lint/task/types.go
git commit -m "refactor: extract action grammar into internal/action"
```

---

### Task 7: Add `internal/config` (`.awiki/config` loader)

**Files:**
- Create: `internal/config/awiki.go`
- Create: `internal/config/awiki_test.go`

- [ ] **Step 1: Inspect `.awiki/config` shape**

The bash callers read it with shell `source` semantics: lines like
`KEY=value`, `#` comments, blanks ignored. No nested sections. Look at
how `scripts/serve.sh`, `scripts/synth.sh`, and `scripts/dataset.sh`
read it for the canonical key set, but the loader is generic.

- [ ] **Step 2: Write the failing test**

Create `internal/config/awiki_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadParsesKeyValueLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	body := "# comment\nAWIKI_AGENT=claude\n\nDATASET_INLINE_THRESHOLD=100\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got["AWIKI_AGENT"] != "claude" {
		t.Fatalf("AWIKI_AGENT: %q", got["AWIKI_AGENT"])
	}
	if got["DATASET_INLINE_THRESHOLD"] != "100" {
		t.Fatalf("DATASET_INLINE_THRESHOLD: %q", got["DATASET_INLINE_THRESHOLD"])
	}
}

func TestLoadMissingFileReturnsEmptyMap(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestLoadStripsInlineComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte("KEY=value  # trailing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got["KEY"] != "value" {
		t.Fatalf("KEY: %q", got["KEY"])
	}
}
```

- [ ] **Step 3: Run the test, verify it fails**

```bash
go test ./internal/config/...
```

Expected: build error.

- [ ] **Step 4: Implement `internal/config/awiki.go`**

```go
// Package config loads the awiki .awiki/config file (shell-style
// KEY=value with `#` comments).
package config

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"strings"
)

// Load reads path and returns its key/value pairs. A missing file
// returns an empty map without error (matches `source -f` semantics
// in the bash scripts that tolerate an absent config).
func Load(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	defer f.Close()
	out := map[string]string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Inline comment: split on the first ` #` (space-hash).
		if i := strings.Index(line, " #"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		val = strings.TrimSpace(val)
		out[key] = val
	}
	return out, scanner.Err()
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/config/...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/config/awiki.go internal/config/awiki_test.go
git commit -m "feat: add internal/config awiki loader"
```

---

### Task 8: Extend `internal/adapters` (typed interface stubs)

**Files:**
- Create: `internal/adapters/interfaces.go`
- Create: `internal/adapters/interfaces_test.go`

This task adds the *interface* layer the roadmap pins. No
implementations yet; first implementation lands per-domain. The point
is to fix the shapes so each domain spec can be written against them.

- [ ] **Step 1: Sketch the interfaces**

Pick one method per adapter, matching the smallest job each domain will
actually call. The roadmap names: hugo, qmd, duckdb, vlconvert,
pdftotext, whisper, git, gpg, fsnotify.

- [ ] **Step 2: Write the failing test (compile-time interface assertions)**

Create `internal/adapters/interfaces_test.go`:

```go
package adapters

import "testing"

func TestInterfaceShapes(t *testing.T) {
	var _ Hugo = (*hugoStub)(nil)
	var _ Qmd = (*qmdStub)(nil)
	var _ DuckDB = (*duckdbStub)(nil)
	var _ VLConvert = (*vlconvertStub)(nil)
	var _ PDFToText = (*pdftotextStub)(nil)
	var _ Whisper = (*whisperStub)(nil)
	var _ Git = (*gitStub)(nil)
	var _ GPG = (*gpgStub)(nil)
	var _ FSNotify = (*fsnotifyStub)(nil)
	t.Log("compile-time interface check ok")
}
```

- [ ] **Step 3: Run the test, verify it fails**

```bash
go test ./internal/adapters/...
```

Expected: build errors (interfaces and stubs undefined).

- [ ] **Step 4: Implement the interfaces and matching no-op stubs**

Create `internal/adapters/interfaces.go`:

```go
package adapters

import "context"

// Hugo runs the Hugo build/check pipeline.
type Hugo interface {
	Check(ctx context.Context, repoRoot string) (output string, code int, err error)
}

// Qmd is the qmd CLI adapter (search/index/install).
type Qmd interface {
	Search(ctx context.Context, repoRoot, query string) (output string, code int, err error)
	Reindex(ctx context.Context, repoRoot string) (output string, code int, err error)
}

// DuckDB runs an SQL query and returns rows as TSV/JSON depending on
// the caller-supplied format string. Used by the query domain.
type DuckDB interface {
	Run(ctx context.Context, sql, format string) (output string, code int, err error)
}

// VLConvert renders a Vega-Lite spec to SVG. Used by the chart domain.
type VLConvert interface {
	RenderSVG(ctx context.Context, specPath, outPath string) (code int, err error)
}

// PDFToText extracts text from a PDF. Used by the ingest-pdf flow.
type PDFToText interface {
	Extract(ctx context.Context, pdfPath string) (text string, code int, err error)
}

// Whisper transcribes audio. Used by the ingest-audio flow.
type Whisper interface {
	Transcribe(ctx context.Context, audioPath string) (text string, code int, err error)
}

// Git wraps the git CLI for operations not covered by go-git.
type Git interface {
	Clone(ctx context.Context, url, dest string) (code int, err error)
	RevParse(ctx context.Context, dir, ref string) (sha string, code int, err error)
}

// GPG wraps the gpg/git-crypt/age CLIs for the encrypt-init lifecycle.
type GPG interface {
	Available(ctx context.Context) (bool, error)
}

// FSNotify is the watch-loop adapter; the implementation may use the
// fsnotify package or fall back to polling.
type FSNotify interface {
	Watch(ctx context.Context, root string, events chan<- string) error
}

// --- no-op stubs used solely for compile-time interface assertions ---

type hugoStub struct{}

func (hugoStub) Check(context.Context, string) (string, int, error) { return "", 0, nil }

type qmdStub struct{}

func (qmdStub) Search(context.Context, string, string) (string, int, error) { return "", 0, nil }
func (qmdStub) Reindex(context.Context, string) (string, int, error)        { return "", 0, nil }

type duckdbStub struct{}

func (duckdbStub) Run(context.Context, string, string) (string, int, error) { return "", 0, nil }

type vlconvertStub struct{}

func (vlconvertStub) RenderSVG(context.Context, string, string) (int, error) { return 0, nil }

type pdftotextStub struct{}

func (pdftotextStub) Extract(context.Context, string) (string, int, error) { return "", 0, nil }

type whisperStub struct{}

func (whisperStub) Transcribe(context.Context, string) (string, int, error) { return "", 0, nil }

type gitStub struct{}

func (gitStub) Clone(context.Context, string, string) (int, error)              { return 0, nil }
func (gitStub) RevParse(context.Context, string, string) (string, int, error)   { return "", 0, nil }

type gpgStub struct{}

func (gpgStub) Available(context.Context) (bool, error) { return false, nil }

type fsnotifyStub struct{}

func (fsnotifyStub) Watch(context.Context, string, chan<- string) error { return nil }
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/adapters/...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/adapters/interfaces.go internal/adapters/interfaces_test.go
git commit -m "feat: add adapter interface stubs for upcoming domain ports"
```

---

### Task 9: Final regression check

**Files:**
- (read-only)

- [ ] **Step 1: Build the binary**

```bash
go build -o bin/awiki ./cmd/awiki
```

Expected: success, no warnings.

- [ ] **Step 2: Run all Go tests**

```bash
go test ./...
```

Expected: PASS across every package, including
`internal/lint/{,synth,task,data,chart,query}`.

- [ ] **Step 3: Run the bats suite via the existing shim**

```bash
just test
```

Expected: PASS. The lint shim continues to drive `awiki lint` for every
existing case; refactor introduced no behavior change.

- [ ] **Step 4: Confirm `just lint` end-to-end on the live wiki**

```bash
just lint
```

Expected: identical record output to pre-refactor (visually inspect; no
record format moved, only where the format string is built).

- [ ] **Step 5: No-op commit only if any cleanup needed**

If steps 1-4 reveal no leftovers (orphan symbols, dead imports), skip
the commit. Otherwise:

```bash
git add -A
git commit -m "chore: tidy after shared-blocks extraction"
```

---

## Self-review checklist

- **Spec coverage.** The roadmap section "Architecture > Shared
  building blocks" lists `fsutil`, `region`, `action`, `config`,
  `emit`, `adapters`. Each maps to a task: fsutil (Tasks 1-2), emit
  (Task 3), region (Tasks 4-5), action (Task 6), config (Task 7),
  adapters (Task 8). Final regression in Task 9.
- **Placeholder scan.** No "TBD"/"TODO"/"implement later" remain. Every
  code block is the actual content the engineer types.
- **Type consistency.** `AtomicWrite`, `WithExclusiveLock`,
  `WithSharedLock`, `ContentionError` (fsutil); `Lint`, `Fix`,
  `LintSummary`, `AgentPrompt`, `Ingest`, `Review`, `Recur` (emit);
  `ParseGenerated`, `Generated`, `Diagnostic`, `ManagedReplace`,
  `ManagedExtract` (region); `ParseLine`, `Action` (action); `Load`
  (config); `Hugo`, `Qmd`, `DuckDB`, `VLConvert`, `PDFToText`,
  `Whisper`, `Git`, `GPG`, `FSNotify` (adapters). The names used in
  test code, implementation, and update sites all match.
- **Out of scope (correctly).** Slice 0 lint cleanup (deleting bash and
  python lint scripts) is its own plan, gated on the release-window
  criterion in the roadmap. Synth and beyond are deferred to per-domain
  specs and plans.
