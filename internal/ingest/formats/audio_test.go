package formats

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"awiki/internal/ingest"
	"awiki/internal/testutil"
)

// audioFixedTime is the deterministic clock the slice-5 fixture-driven
// test injects. Format matches the bash `date '+%Y-%m-%d'` form used
// for the frontmatter `date` and `last_updated` fields.
var audioFixedTime = time.Date(2026, 4, 30, 12, 34, 0, 0, time.UTC)

// audioFixtureRoot resolves the repo-relative path to the slice 5
// fixture tree. Mirrors pdfFixtureRoot.
func audioFixtureRoot(t *testing.T) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(here), "..", "..", ".."))
	return filepath.Join(repoRoot, "tests", "fixtures", "ingest", "audio")
}

// TestIngestAudioFixture drives formats.IngestAudio against the slice-5
// golden fixture. The fixture pins the AUDIO-TRANSCRIBED record, the
// rendered markdown body + frontmatter, and the stashed original.
func TestIngestAudioFixture(t *testing.T) {
	fixture := audioFixtureRoot(t)

	tmp := t.TempDir()
	testutil.CopyTree(t, filepath.Join(fixture, "input"), tmp)

	argsBytes, err := os.ReadFile(filepath.Join(fixture, "args"))
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	args := strings.Fields(strings.TrimSpace(string(argsBytes)))
	if len(args) < 2 || args[0] != "ingest-audio" {
		t.Fatalf("args fixture must start with `ingest-audio`, got %q", argsBytes)
	}
	srcArg := args[1]

	r := &ingest.Runner{
		RepoRoot: tmp,
		Today:    audioFixedTime.Format("2006-01-02"),
		Whisper: &testutil.FakeWhisper{
			TextPath: filepath.Join(tmp, "canned-text.txt"),
		},
	}

	var stdout, stderr bytes.Buffer
	res, err := IngestAudio(context.Background(), r, AudioOptions{SourcePath: srcArg}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("IngestAudio: %v\nstderr: %s", err, stderr.String())
	}
	if res == nil {
		t.Fatal("IngestAudio returned nil result")
	}

	wantStdout, err := os.ReadFile(filepath.Join(fixture, "expected_stdout"))
	if err != nil {
		t.Fatalf("read expected_stdout: %v", err)
	}
	if got := stdout.String(); got != string(wantStdout) {
		t.Errorf("stdout mismatch:\n got: %q\nwant: %q", got, string(wantStdout))
	}

	if _, err := os.Stat(filepath.Join(tmp, srcArg)); !os.IsNotExist(err) {
		t.Errorf("expected source audio to be removed, stat err = %v", err)
	}

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
// Behavioral tests covering the bash-compat guards.
// --------------------------------------------------------------------

func newAudioRunner(t *testing.T, cannedText string) (*ingest.Runner, string) {
	t.Helper()
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "raw", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := &ingest.Runner{
		RepoRoot: tmp,
		Today:    audioFixedTime.Format("2006-01-02"),
		Whisper:  &testutil.FakeWhisper{Text: cannedText},
	}
	return r, tmp
}

func TestIngestAudioRejectsMissingFile(t *testing.T) {
	r, _ := newAudioRunner(t, "x")
	var stdout, stderr bytes.Buffer
	_, err := IngestAudio(context.Background(), r, AudioOptions{SourcePath: "raw/inbox/missing.m4a"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	var ee *ingest.ExitError
	if !errors.As(err, &ee) || ee.Code != 1 {
		t.Fatalf("expected ExitError code 1, got %v", err)
	}
	if !strings.Contains(stderr.String(), "not found") {
		t.Errorf("stderr missing 'not found': %q", stderr.String())
	}
}

func TestIngestAudioRejectsPathOutsideInbox(t *testing.T) {
	r, tmp := newAudioRunner(t, "x")
	if err := os.MkdirAll(filepath.Join(tmp, "elsewhere"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "elsewhere", "x.mp3"), []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	_, err := IngestAudio(context.Background(), r, AudioOptions{SourcePath: "elsewhere/x.mp3"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for path outside raw/inbox/, got nil")
	}
	var ee *ingest.ExitError
	if !errors.As(err, &ee) || ee.Code != 1 {
		t.Fatalf("expected ExitError code 1, got %v", err)
	}
	if !strings.Contains(stderr.String(), "must be under raw/inbox/") {
		t.Errorf("stderr missing inbox guard: %q", stderr.String())
	}
}

func TestIngestAudioAdapterFailureExitCode2(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "raw", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "raw", "inbox", "src.wav"), []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &ingest.Runner{
		RepoRoot: tmp,
		Today:    audioFixedTime.Format("2006-01-02"),
		Whisper:  &testutil.FakeWhisper{Code: 1, Err: errors.New("boom")},
	}
	var stdout, stderr bytes.Buffer
	_, err := IngestAudio(context.Background(), r, AudioOptions{SourcePath: "raw/inbox/src.wav"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected adapter failure, got nil")
	}
	var ee *ingest.ExitError
	if !errors.As(err, &ee) || ee.Code != 2 {
		t.Fatalf("expected ExitError code 2, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "raw", "inbox", "src.wav")); err != nil {
		t.Errorf("source audio should remain on adapter failure: %v", err)
	}
}

// TestIngestAudioStripsArbitraryExtension confirms `${AUDIO%.*}.md`
// semantics: any trailing extension is stripped. PDF rejects non-.pdf
// inputs; audio accepts anything and just rewrites the suffix.
func TestIngestAudioStripsArbitraryExtension(t *testing.T) {
	r, tmp := newAudioRunner(t, "transcript")
	if err := os.WriteFile(filepath.Join(tmp, "raw", "inbox", "memo.flac"), []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	res, err := IngestAudio(context.Background(), r, AudioOptions{SourcePath: "raw/inbox/memo.flac"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("IngestAudio: %v\nstderr: %s", err, stderr.String())
	}
	if res.DestPath != "raw/inbox/memo.md" {
		t.Errorf("dest = %q, want raw/inbox/memo.md", res.DestPath)
	}
}
