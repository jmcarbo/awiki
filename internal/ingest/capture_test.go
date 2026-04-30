package ingest

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"awiki/internal/testutil"
)

// fixedTime is the deterministic clock the fixture-driven tests inject.
// Format matches the bash `date '+%Y-%m-%d %H:%M'` form.
var fixedTime = time.Date(2026, 4, 30, 12, 34, 0, 0, time.UTC)

// captureFixtureRoot resolves the repo-relative path to the slice 2
// fixture tree. The Go test binary runs with cwd = the package dir, so
// we walk up from runtime.Caller to the repo root.
func captureFixtureRoot(t *testing.T) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	// internal/ingest/capture_test.go -> repo root is two levels up.
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(here), "..", ".."))
	return filepath.Join(repoRoot, "tests", "fixtures", "ingest", "capture")
}

// TestCaptureFixture drives Runner.Capture against the golden fixture
// at tests/fixtures/ingest/capture/. The fixture pins:
//   - input/    seed tree copied into a temp RepoRoot before invoke.
//   - args      argv-style line "-- <text...>" the dispatcher would parse.
//   - expected_stdout  byte-for-byte stdout (timestamp normalized via NowFn).
//   - expected_files/  every file produced or modified, byte-equivalent.
func TestCaptureFixture(t *testing.T) {
	fixture := captureFixtureRoot(t)

	tmp := t.TempDir()
	testutil.CopyTree(t, filepath.Join(fixture, "input"), tmp)

	argsBytes, err := os.ReadFile(filepath.Join(fixture, "args"))
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	args := strings.Fields(strings.TrimSpace(string(argsBytes)))
	if len(args) < 2 || args[0] != "--" {
		t.Fatalf("args fixture must start with `--`, got %q", argsBytes)
	}

	r := &Runner{
		RepoRoot: tmp,
		NowFn:    func() time.Time { return fixedTime },
	}

	var stdout, stderr bytes.Buffer
	err = r.Capture(CaptureOptions{Text: strings.Join(args[1:], " ")}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Capture: %v\nstderr: %s", err, stderr.String())
	}

	wantStdout, err := os.ReadFile(filepath.Join(fixture, "expected_stdout"))
	if err != nil {
		t.Fatalf("read expected_stdout: %v", err)
	}
	if got := stdout.String(); got != string(wantStdout) {
		t.Errorf("stdout mismatch:\n got: %q\nwant: %q", got, string(wantStdout))
	}

	// Walk expected_files/ and compare each file byte-for-byte.
	expectedRoot := filepath.Join(fixture, "expected_files")
	if err := filepath.Walk(expectedRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(expectedRoot, path)
		if err != nil {
			return err
		}
		got, err := os.ReadFile(filepath.Join(tmp, rel))
		if err != nil {
			t.Errorf("missing expected file %s: %v", rel, err)
			return nil
		}
		want, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Equal(got, want) {
			t.Errorf("file %s mismatch\n got: %q\nwant: %q", rel, got, want)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk expected_files: %v", err)
	}
}

// --------------------------------------------------------------------
// Behavioral tests covering the bash-compat sanitization table. These
// run in addition to the fixture so each branch in capture.go is pinned.
// They mirror the cases in tests/capture_test.bats so the bats oracle
// can be deleted in slice 10 without coverage loss.
// --------------------------------------------------------------------

// newCaptureRunner returns a Runner rooted at a fresh tempdir with the
// canonical inbox scaffold pre-written. Mirrors capture_test.bats setup.
func newCaptureRunner(t *testing.T) (*Runner, string) {
	t.Helper()
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "content"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "content", "inbox.md"), []byte(inboxFrontmatter), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &Runner{
		RepoRoot: tmp,
		NowFn:    func() time.Time { return fixedTime },
	}
	return r, tmp
}

func TestCaptureEmptyText(t *testing.T) {
	r, _ := newCaptureRunner(t)
	var stdout, stderr bytes.Buffer
	err := r.Capture(CaptureOptions{Text: ""}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for empty text, got nil")
	}
	var ee *ExitError
	if !errors.As(err, &ee) || ee.Code != 4 {
		t.Fatalf("expected ExitError code 4, got %v", err)
	}
	if !strings.Contains(stderr.String(), "ERROR|empty") {
		t.Errorf("stderr missing ERROR|empty: %q", stderr.String())
	}
}

func TestCaptureControlChar(t *testing.T) {
	r, _ := newCaptureRunner(t)
	var stdout, stderr bytes.Buffer
	err := r.Capture(CaptureOptions{Text: "line one\nline two"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for control char, got nil")
	}
	var ee *ExitError
	if !errors.As(err, &ee) || ee.Code != 4 {
		t.Fatalf("expected ExitError code 4, got %v", err)
	}
	if !strings.Contains(stderr.String(), "ERROR|control") {
		t.Errorf("stderr missing ERROR|control: %q", stderr.String())
	}
}

func TestCaptureCheckboxRejected(t *testing.T) {
	for _, s := range []string{"[ ]", "[/]", "[?]", "[>]", "[x]", "[-]"} {
		t.Run(s, func(t *testing.T) {
			r, _ := newCaptureRunner(t)
			var stdout, stderr bytes.Buffer
			err := r.Capture(CaptureOptions{Text: s + " foo"}, &stdout, &stderr)
			if err == nil {
				t.Fatalf("expected error for checkbox %s, got nil", s)
			}
			var ee *ExitError
			if !errors.As(err, &ee) || ee.Code != 4 {
				t.Fatalf("expected ExitError code 4, got %v", err)
			}
			if !strings.Contains(stderr.String(), "ERROR|checkbox") {
				t.Errorf("stderr missing ERROR|checkbox: %q", stderr.String())
			}
		})
	}
}

func TestCaptureLengthTruncated(t *testing.T) {
	r, tmp := newCaptureRunner(t)
	long := strings.Repeat("a", 2050)
	var stdout, stderr bytes.Buffer
	if err := r.Capture(CaptureOptions{Text: long}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "length-truncated") {
		t.Errorf("stderr missing length-truncated: %q", stderr.String())
	}
	body, _ := os.ReadFile(filepath.Join(tmp, "content", "inbox.md"))
	if !strings.HasSuffix(strings.TrimRight(string(body), "\n"), "…") {
		t.Errorf("expected last line to end in …, got %q", body)
	}
}

func TestCaptureWikilinkNeutralized(t *testing.T) {
	r, tmp := newCaptureRunner(t)
	var stdout, stderr bytes.Buffer
	if err := r.Capture(CaptureOptions{Text: "see [[s-as-we-may-think]] later"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "wikilink-neutralized") {
		t.Errorf("stderr missing wikilink-neutralized: %q", stderr.String())
	}
	body, _ := os.ReadFile(filepath.Join(tmp, "content", "inbox.md"))
	if !strings.Contains(string(body), "[ [s-as-we-may-think] ]") {
		t.Errorf("inbox missing neutralized wikilink: %q", body)
	}
	if strings.Contains(string(body), "[[s-as-we-may-think]]") {
		t.Errorf("inbox still contains raw wikilink: %q", body)
	}
}

func TestCaptureCommentNeutralized(t *testing.T) {
	r, tmp := newCaptureRunner(t)
	var stdout, stderr bytes.Buffer
	if err := r.Capture(CaptureOptions{Text: "watch out <!-- inside --> here"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "comment-neutralized") {
		t.Errorf("stderr missing comment-neutralized: %q", stderr.String())
	}
	body, _ := os.ReadFile(filepath.Join(tmp, "content", "inbox.md"))
	if !strings.Contains(string(body), "< !--") {
		t.Errorf("inbox missing neutralized open comment: %q", body)
	}
	if !strings.Contains(string(body), "--  >") {
		t.Errorf("inbox missing neutralized close comment: %q", body)
	}
}

func TestCaptureBlockIDEscaped(t *testing.T) {
	r, tmp := newCaptureRunner(t)
	var stdout, stderr bytes.Buffer
	if err := r.Capture(CaptureOptions{Text: "remember ^abc1234 token"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "block-id-escaped") {
		t.Errorf("stderr missing block-id-escaped: %q", stderr.String())
	}
	body, _ := os.ReadFile(filepath.Join(tmp, "content", "inbox.md"))
	if !strings.Contains(string(body), `\^abc1234`) {
		t.Errorf("inbox missing escaped block-id: %q", body)
	}
}

func TestCaptureCleanLineNoSanitization(t *testing.T) {
	r, _ := newCaptureRunner(t)
	var stdout, stderr bytes.Buffer
	if err := r.Capture(CaptureOptions{Text: "totally normal text"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stderr.String(), "SANITIZATION-APPLIED") {
		t.Errorf("clean line should not emit SANITIZATION-APPLIED, got %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "OK|appended|") {
		t.Errorf("stdout missing OK|appended|: %q", stdout.String())
	}
}

func TestCaptureCreatesInboxIfMissing(t *testing.T) {
	tmp := t.TempDir()
	r := &Runner{RepoRoot: tmp, NowFn: func() time.Time { return fixedTime }}
	var stdout, stderr bytes.Buffer
	if err := r.Capture(CaptureOptions{Text: "first capture"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(tmp, "content", "inbox.md"))
	if err != nil {
		t.Fatalf("inbox not created: %v", err)
	}
	if !strings.HasPrefix(string(body), "---\n") {
		t.Errorf("inbox missing frontmatter scaffold: %q", body)
	}
	if !strings.Contains(string(body), "type: inbox") {
		t.Errorf("inbox missing type:inbox: %q", body)
	}
	if !strings.Contains(string(body), "first capture") {
		t.Errorf("inbox missing capture line: %q", body)
	}
}

func TestCaptureTabConvertsToSpace(t *testing.T) {
	r, tmp := newCaptureRunner(t)
	var stdout, stderr bytes.Buffer
	if err := r.Capture(CaptureOptions{Text: "with\ttab"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(tmp, "content", "inbox.md"))
	if !strings.Contains(string(body), "with tab") {
		t.Errorf("expected tab to convert to space: %q", body)
	}
	if strings.Contains(stderr.String(), "SANITIZATION-APPLIED") {
		t.Errorf("tab conversion should not count as a sanitization, stderr: %q", stderr.String())
	}
}

func TestCapturePresanitizedSkipsSanitization(t *testing.T) {
	r, tmp := newCaptureRunner(t)
	var stdout, stderr bytes.Buffer
	// Wikilink would normally be neutralized; presanitized=true keeps it.
	if err := r.Capture(CaptureOptions{Text: "see [[slug]] now", Presanitized: true}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stderr.String(), "SANITIZATION-APPLIED") {
		t.Errorf("presanitized should suppress SANITIZATION-APPLIED: %q", stderr.String())
	}
	body, _ := os.ReadFile(filepath.Join(tmp, "content", "inbox.md"))
	if !strings.Contains(string(body), "[[slug]]") {
		t.Errorf("presanitized should keep raw wikilink: %q", body)
	}
}

func TestCaptureInboxPathOverride(t *testing.T) {
	tmp := t.TempDir()
	custom := filepath.Join(tmp, "alt", "inbox.md")
	r := &Runner{RepoRoot: tmp, NowFn: func() time.Time { return fixedTime }}
	var stdout, stderr bytes.Buffer
	if err := r.Capture(CaptureOptions{Text: "custom inbox", InboxPath: custom}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(custom); err != nil {
		t.Errorf("custom inbox not created: %v", err)
	}
}
