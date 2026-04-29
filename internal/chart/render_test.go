package chart

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeVLConvert is a test double that writes a synthetic SVG file and
// returns exit code 0.
type fakeVLConvert struct {
	// failIDs is the set of chart IDs that should fail (matched by output path base).
	failIDs map[string]bool
}

func (f *fakeVLConvert) RenderSVG(_ context.Context, specPath, outPath string) (int, error) {
	id := strings.TrimSuffix(filepath.Base(outPath), ".svg")
	if f.failIDs != nil && f.failIDs[id] {
		return 1, &testRenderError{msg: "vl-convert failed for " + id}
	}
	svg := []byte("<svg>" + id + "</svg>")
	if err := os.WriteFile(outPath, svg, 0644); err != nil {
		return 1, err
	}
	return 0, nil
}

type testRenderError struct{ msg string }

func (e *testRenderError) Error() string { return e.msg }

// makeRenderRepo creates a full test repo with a dataset and a chart page.
func makeRenderRepo(t *testing.T) (root string, runner *Runner) {
	t.Helper()
	root = t.TempDir()

	// Create dataset.
	dsDir := filepath.Join(root, "content", "datasets")
	if err := os.MkdirAll(dsDir, 0755); err != nil {
		t.Fatal(err)
	}
	dsContent := `---
title: "sales"
type: dataset
storage: inline
format: csv
---

## Data

` + "```csv" + `
month,revenue
Jan,100
Feb,200
` + "```" + `
`
	if err := os.WriteFile(filepath.Join(dsDir, "sales.md"), []byte(dsContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create chart page.
	chartsDir := filepath.Join(root, "content", "charts")
	if err := os.MkdirAll(chartsDir, 0755); err != nil {
		t.Fatal(err)
	}
	chartContent := `---
title: "revenue"
type: chart
chart_engine: vega-lite
---

# revenue

` + "```vega-lite" + `
{"mark": "bar", "data": {"name": "[[sales]]"}}
` + "```" + `
`
	if err := os.WriteFile(filepath.Join(chartsDir, "revenue.md"), []byte(chartContent), 0644); err != nil {
		t.Fatal(err)
	}

	runner = &Runner{
		RepoRoot:   root,
		ContentDir: filepath.Join(root, "content"),
		AssetsDir:  filepath.Join(root, "assets", "charts"),
		VLConvert:  &fakeVLConvert{},
	}
	return root, runner
}

func TestRender_WalkAndExtractAndRender(t *testing.T) {
	root, runner := makeRenderRepo(t)

	var stdout, stderr bytes.Buffer
	if err := runner.Render(RenderOptions{}, &stdout, &stderr); err != nil {
		t.Fatalf("Render error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "CHART|rendered revenue") {
		t.Errorf("expected 'CHART|rendered revenue' in stdout:\n%s", out)
	}

	// SVG sidecar should exist.
	svgPath := filepath.Join(root, "assets", "charts", "revenue.svg")
	if _, err := os.Stat(svgPath); err != nil {
		t.Errorf("SVG sidecar should exist at %s: %v", svgPath, err)
	}

	// Hash sidecar should exist.
	hashPath := filepath.Join(root, "assets", "charts", "revenue.svg.hash")
	if _, err := os.Stat(hashPath); err != nil {
		t.Errorf("hash sidecar should exist: %v", err)
	}

	// Chart page should have preview region injected.
	b, err := os.ReadFile(filepath.Join(root, "content", "charts", "revenue.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "<!-- BEGIN chart-preview:revenue -->") {
		t.Errorf("expected preview region in chart page:\n%s", string(b))
	}
}

func TestRender_SkipOnHashMatch(t *testing.T) {
	_, runner := makeRenderRepo(t)

	var stdout bytes.Buffer
	// First render.
	if err := runner.Render(RenderOptions{KeepOrphans: true}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()

	// Second render — should skip.
	if err := runner.Render(RenderOptions{KeepOrphans: true}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "CHART|skip revenue (hash match)") {
		t.Errorf("expected skip on second render, got:\n%s", out)
	}
}

func TestRender_OrphanCleanup(t *testing.T) {
	root, runner := makeRenderRepo(t)

	// Pre-create an orphan sidecar.
	assetsDir := filepath.Join(root, "assets", "charts")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatal(err)
	}
	orphanSVG := filepath.Join(assetsDir, "ghost.svg")
	orphanHash := filepath.Join(assetsDir, "ghost.svg.hash")
	if err := os.WriteFile(orphanSVG, []byte("<svg/>"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(orphanHash, []byte("abc"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	if err := runner.Render(RenderOptions{}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "CHART|removed orphan") {
		t.Errorf("expected orphan removal message, got:\n%s", out)
	}
	if _, err := os.Stat(orphanSVG); !os.IsNotExist(err) {
		t.Error("orphan SVG should have been removed")
	}
}

func TestRender_PreviewInjection(t *testing.T) {
	root, runner := makeRenderRepo(t)

	var stdout bytes.Buffer
	if err := runner.Render(RenderOptions{KeepOrphans: true}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(filepath.Join(root, "content", "charts", "revenue.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	if !strings.Contains(text, "BEGIN chart-preview:revenue") {
		t.Error("expected managed region in page")
	}
	if !strings.Contains(text, "revenue.svg") {
		t.Error("expected SVG link in preview region")
	}
}

func TestRender_FailedRender(t *testing.T) {
	root, runner := makeRenderRepo(t)
	runner.VLConvert = &fakeVLConvert{failIDs: map[string]bool{"revenue": true}}

	var stdout, stderr bytes.Buffer
	_ = runner.Render(RenderOptions{KeepOrphans: true}, &stdout, &stderr)

	out := stdout.String()
	if !strings.Contains(out, "CHART|RENDER|revenue|") {
		t.Errorf("expected CHART|RENDER|revenue| in stdout:\n%s", out)
	}

	// Failed file should exist.
	failedPath := filepath.Join(root, "assets", "charts", "revenue.svg.failed")
	if _, err := os.Stat(failedPath); err != nil {
		t.Errorf("expected .svg.failed sidecar: %v", err)
	}
}

func TestRender_MultiFence(t *testing.T) {
	root := t.TempDir()

	// Dataset.
	dsDir := filepath.Join(root, "content", "datasets")
	if err := os.MkdirAll(dsDir, 0755); err != nil {
		t.Fatal(err)
	}
	ds := `---
title: "ds"
type: dataset
storage: inline
format: csv
---

## Data

` + "```csv" + `
x,y
1,2
` + "```" + `
`
	if err := os.WriteFile(filepath.Join(dsDir, "ds.md"), []byte(ds), 0644); err != nil {
		t.Fatal(err)
	}

	// Page with 3 fences (not a chart page).
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

Some text.

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
	if err := runner.Render(RenderOptions{KeepOrphans: true}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "overview-fig0") {
		t.Errorf("expected overview-fig0, got:\n%s", out)
	}
	if !strings.Contains(out, "overview-fig1") {
		t.Errorf("expected overview-fig1, got:\n%s", out)
	}
}

func TestRender_JSONSidecar(t *testing.T) {
	root, runner := makeRenderRepo(t)

	if err := runner.Render(RenderOptions{KeepOrphans: true}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}

	jsonPath := filepath.Join(root, "assets", "charts", "revenue.json")
	b, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("JSON sidecar missing: %v", err)
	}
	var spec map[string]any
	if err := json.Unmarshal(b, &spec); err != nil {
		t.Fatalf("JSON sidecar invalid: %v", err)
	}
	// Should have resolved data (values, not name).
	data, ok := spec["data"].(map[string]any)
	if !ok {
		t.Fatal("data key missing or wrong type")
	}
	if _, ok := data["values"]; !ok {
		t.Error("expected resolved 'values' in data")
	}
}
