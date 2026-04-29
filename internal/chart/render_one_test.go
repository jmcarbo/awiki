package chart

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderOne_LocateAndRender(t *testing.T) {
	root, runner := makeRenderRepo(t)

	var stdout, stderr bytes.Buffer
	if err := runner.RenderOne("revenue", &stdout, &stderr); err != nil {
		t.Fatalf("RenderOne error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "CHART|rendered revenue") {
		t.Errorf("expected 'CHART|rendered revenue', got:\n%s", out)
	}

	// SVG should exist.
	if _, err := os.Stat(filepath.Join(root, "assets", "charts", "revenue.svg")); err != nil {
		t.Errorf("SVG sidecar should exist: %v", err)
	}
}

func TestRenderOne_NotFound(t *testing.T) {
	_, runner := makeRenderRepo(t)

	var stdout, stderr bytes.Buffer
	err := runner.RenderOne("nonexistent-chart", &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for unknown chart ID")
	}
	if !strings.Contains(err.Error(), "unknown chart") {
		t.Errorf("want 'unknown chart' in error, got %q", err.Error())
	}
}

func TestRenderOne_SkipOnHashMatch(t *testing.T) {
	_, runner := makeRenderRepo(t)

	// First render.
	if err := runner.RenderOne("revenue", &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}

	// Second render — should skip.
	var stdout bytes.Buffer
	if err := runner.RenderOne("revenue", &stdout, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "CHART|skip revenue (hash match)") {
		t.Errorf("expected hash-match skip on second render, got:\n%s", out)
	}
}

func TestRenderOne_SelectsCorrectFence(t *testing.T) {
	root := t.TempDir()

	// Create dataset.
	dsDir := filepath.Join(root, "content", "datasets")
	if err := os.MkdirAll(dsDir, 0755); err != nil {
		t.Fatal(err)
	}
	dsContent := `---
title: "ds"
type: dataset
storage: inline
format: csv
---

## Data

` + "```csv" + `
a,b
1,2
` + "```" + `
`
	if err := os.WriteFile(filepath.Join(dsDir, "ds.md"), []byte(dsContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Page with 2 fences — IDs are overview-fig0 and overview-fig1.
	pagesDir := filepath.Join(root, "content", "pages")
	if err := os.MkdirAll(pagesDir, 0755); err != nil {
		t.Fatal(err)
	}
	pg := `---
title: "overview"
type: note
---

` + "```vega-lite" + `
{"mark": "bar", "data": {"name": "[[ds]]"}}
` + "```" + `

` + "```vega-lite" + `
{"mark": "line", "data": {"name": "[[ds]]"}}
` + "```" + `
`
	if err := os.WriteFile(filepath.Join(pagesDir, "overview.md"), []byte(pg), 0644); err != nil {
		t.Fatal(err)
	}

	runner := &Runner{
		RepoRoot:   root,
		ContentDir: filepath.Join(root, "content"),
		AssetsDir:  filepath.Join(root, "assets", "charts"),
		VLConvert:  &fakeVLConvert{},
	}

	var stdout bytes.Buffer
	// Render only the second fence.
	if err := runner.RenderOne("overview-fig1", &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "overview-fig1") {
		t.Errorf("expected overview-fig1 rendered, got:\n%s", out)
	}
	// overview-fig0 should NOT be rendered.
	if strings.Contains(out, "overview-fig0") {
		t.Errorf("should not render overview-fig0, got:\n%s", out)
	}
}
