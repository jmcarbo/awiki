package synth

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validMindmapPage = `---
title: foo
---

# Mindmap

<!-- BEGIN GENERATED foo -->

` + "```mermaid" + `
mindmap
  root((Topic))
    Cluster A
      Leaf 1
      Leaf 2
    Cluster B
      Leaf 3
` + "```" + `

<!-- END GENERATED -->
`

func writeMindmapPage(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestMindmapValidateMissingArg(t *testing.T) {
	var stderr bytes.Buffer
	code := MindmapValidate(context.Background(), MindmapValidateOptions{}, &stderr)
	if code != MindmapValidateUsage {
		t.Fatalf("code = %d, want %d", code, MindmapValidateUsage)
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestMindmapValidateMissingPage(t *testing.T) {
	var stderr bytes.Buffer
	code := MindmapValidate(context.Background(), MindmapValidateOptions{
		Page:       "/no/such/page.md",
		MmdcLookup: func(string) (string, error) { return "", errors.New("absent") },
	}, &stderr)
	if code != MindmapValidateMissingPage {
		t.Fatalf("code = %d, want %d", code, MindmapValidateMissingPage)
	}
	if !strings.Contains(stderr.String(), "page not found") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestMindmapValidateNoMermaidBlock(t *testing.T) {
	page := writeMindmapPage(t, "p.md", "no generated region here\n")
	var stderr bytes.Buffer
	code := MindmapValidate(context.Background(), MindmapValidateOptions{
		Page:       page,
		MmdcLookup: func(string) (string, error) { return "", errors.New("absent") },
	}, &stderr)
	if code != MindmapValidateNoMermaidBlock {
		t.Fatalf("code = %d, want %d", code, MindmapValidateNoMermaidBlock)
	}
}

func TestMindmapValidateBadFirstLineWithoutMmdc(t *testing.T) {
	body := strings.Replace(validMindmapPage, "mindmap\n", "flowchart TD\n", 1)
	page := writeMindmapPage(t, "bad.md", body)
	var stderr bytes.Buffer
	code := MindmapValidate(context.Background(), MindmapValidateOptions{
		Page:       page,
		MmdcLookup: func(string) (string, error) { return "", errors.New("absent") },
	}, &stderr)
	if code != MindmapValidateBadFirstLine {
		t.Fatalf("code = %d, want %d, stderr=%q", code, MindmapValidateBadFirstLine, stderr.String())
	}
}

func TestMindmapValidateBraceImbalanceWithoutMmdc(t *testing.T) {
	body := strings.Replace(validMindmapPage, "root((Topic))", "root((Topic)", 1)
	page := writeMindmapPage(t, "imbalanced.md", body)
	var stderr bytes.Buffer
	code := MindmapValidate(context.Background(), MindmapValidateOptions{
		Page:       page,
		MmdcLookup: func(string) (string, error) { return "", errors.New("absent") },
	}, &stderr)
	if code != MindmapValidateBraceImbalance {
		t.Fatalf("code = %d, want %d, stderr=%q", code, MindmapValidateBraceImbalance, stderr.String())
	}
}

func TestMindmapValidatePassesWithoutMmdc(t *testing.T) {
	page := writeMindmapPage(t, "ok.md", validMindmapPage)
	var stderr bytes.Buffer
	code := MindmapValidate(context.Background(), MindmapValidateOptions{
		Page:       page,
		MmdcLookup: func(string) (string, error) { return "", errors.New("absent") },
	}, &stderr)
	if code != MindmapValidateOK {
		t.Fatalf("code = %d, want %d, stderr=%q", code, MindmapValidateOK, stderr.String())
	}
}

func TestMindmapValidateMmdcSuccess(t *testing.T) {
	page := writeMindmapPage(t, "ok.md", validMindmapPage)
	var stderr bytes.Buffer
	called := 0
	code := MindmapValidate(context.Background(), MindmapValidateOptions{
		Page:       page,
		MmdcLookup: func(string) (string, error) { return "/usr/bin/mmdc", nil },
		MmdcRunner: func(_ context.Context, _, _ string) error { called++; return nil },
	}, &stderr)
	if code != MindmapValidateOK {
		t.Fatalf("code = %d, want %d", code, MindmapValidateOK)
	}
	if called != 1 {
		t.Fatalf("MmdcRunner called %d times, want 1", called)
	}
}

func TestMindmapValidateMmdcRejection(t *testing.T) {
	page := writeMindmapPage(t, "ok.md", validMindmapPage)
	var stderr bytes.Buffer
	code := MindmapValidate(context.Background(), MindmapValidateOptions{
		Page:       page,
		MmdcLookup: func(string) (string, error) { return "/usr/bin/mmdc", nil },
		MmdcRunner: func(_ context.Context, _, _ string) error { return errors.New("boom") },
	}, &stderr)
	if code != MindmapValidateMmdcRejected {
		t.Fatalf("code = %d, want %d, stderr=%q", code, MindmapValidateMmdcRejected, stderr.String())
	}
}

func TestMindmapValidateWritesPostHookSentinel(t *testing.T) {
	page := writeMindmapPage(t, "ok.md", validMindmapPage)
	repo := t.TempDir()
	var stderr bytes.Buffer
	code := MindmapValidate(context.Background(), MindmapValidateOptions{
		Page:       page,
		RepoRoot:   repo,
		MmdcLookup: func(string) (string, error) { return "", errors.New("absent") },
	}, &stderr)
	if code != MindmapValidateOK {
		t.Fatalf("code = %d, want 0, stderr=%q", code, stderr.String())
	}
	sentinel := filepath.Join(repo, ".awiki", "post-hook-ran")
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("sentinel not written: %v", err)
	}
}
