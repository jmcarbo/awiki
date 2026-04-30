package ops

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestLogAppendsCanonicalLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.md")
	if err := os.WriteFile(path, []byte("---\ntitle: Log\ntype: log\ndraft: true\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Log(LogOptions{Action: "ingest", Message: "Sample article", LogFile: path}); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	re := regexp.MustCompile(`(?m)^## \[\d{4}-\d{2}-\d{2} \d{2}:\d{2}\] ingest \| Sample article$`)
	if !re.Match(body) {
		t.Fatalf("missing canonical line: %s", body)
	}
}

func TestLogCreatesFileWithHeaderWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.md")
	if _, err := Log(LogOptions{Action: "manual", Message: "first", LogFile: path}); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	if !bytes.HasPrefix(body, []byte("---\ntitle: Log\ntype: log\ndraft: true\n---\n\n")) {
		t.Fatalf("missing canonical header: %q", body)
	}
}

func TestLogSanitizesPipeInMessage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.md")
	if _, err := Log(LogOptions{Action: "triage", Message: "act | _loose | phantom", LogFile: path}); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	// Exactly one '|' on the appended line (the field separator).
	for line := range strings.SplitSeq(string(body), "\n") {
		if !strings.HasPrefix(line, "## [") {
			continue
		}
		if got := strings.Count(line, "|"); got != 1 {
			t.Fatalf("expected 1 pipe, got %d in %q", got, line)
		}
		if !strings.Contains(line, "act _ _loose _ phantom") {
			t.Fatalf("pipe not replaced with underscore in: %q", line)
		}
	}
}

func TestLogSanitizesNewlineAndCR(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.md")
	if _, err := Log(LogOptions{Action: "triage", Message: "first\nsecond", LogFile: path}); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), "first second") {
		t.Fatalf("LF not replaced: %q", body)
	}
	if _, err := Log(LogOptions{Action: "triage", Message: "first\rsecond", LogFile: path}); err != nil {
		t.Fatal(err)
	}
	body, _ = os.ReadFile(path)
	if strings.ContainsRune(string(body), '\r') {
		t.Fatalf("raw CR in log: %q", body)
	}
}

func TestLogSanitizesPipeInAction(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.md")
	if _, err := Log(LogOptions{Action: "evil|action", Message: "msg", LogFile: path}); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	for line := range strings.SplitSeq(string(body), "\n") {
		if !strings.HasPrefix(line, "## [") {
			continue
		}
		if !strings.Contains(line, "evil_action") {
			t.Fatalf("action pipe not replaced: %q", line)
		}
	}
}

func TestLogTimestampFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.md")
	now := time.Date(2026, 4, 30, 13, 5, 0, 0, time.Local)
	if _, err := Log(LogOptions{Action: "ts", Message: "x", LogFile: path, Now: now}); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), "## [2026-04-30 13:05] ts | x\n") {
		t.Fatalf("timestamp shape wrong: %q", body)
	}
}

func TestLogCLIMissingAction(t *testing.T) {
	var stderr bytes.Buffer
	code := LogCLI(nil, nil, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero")
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("expected usage in stderr, got %q", stderr.String())
	}
}
