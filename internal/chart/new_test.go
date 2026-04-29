package chart

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeRunnerWithDataset(t *testing.T, dataSlug string) (*Runner, string) {
	t.Helper()
	root := t.TempDir()
	dsDir := filepath.Join(root, "content", "datasets")
	if err := os.MkdirAll(dsDir, 0755); err != nil {
		t.Fatal(err)
	}
	dsPage := filepath.Join(dsDir, dataSlug+".md")
	if err := os.WriteFile(dsPage, []byte("---\ntitle: \""+dataSlug+"\"\ntype: dataset\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runner := &Runner{
		RepoRoot:   root,
		ContentDir: filepath.Join(root, "content"),
		AssetsDir:  filepath.Join(root, "assets", "charts"),
	}
	return runner, root
}

func TestNew_ScaffoldGolden(t *testing.T) {
	runner, root := makeRunnerWithDataset(t, "sales")
	var buf bytes.Buffer
	err := runner.New(NewOptions{Slug: "revenue", DataSlug: "sales"}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify output record.
	out := buf.String()
	expectedPath := filepath.Join(root, "content", "charts", "revenue.md")
	if !strings.Contains(out, "CHART|created "+expectedPath) {
		t.Errorf("expected CHART|created record, got: %q", out)
	}

	// Verify file content.
	b, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("page not created: %v", err)
	}
	content := string(b)
	if !strings.Contains(content, `title: "revenue"`) {
		t.Error("missing title in scaffold")
	}
	if !strings.Contains(content, `type: chart`) {
		t.Error("missing type:chart in scaffold")
	}
	if !strings.Contains(content, `chart_engine: vega-lite`) {
		t.Error("missing chart_engine in scaffold")
	}
	if !strings.Contains(content, `[[sales]]`) {
		t.Error("missing dataset reference in scaffold")
	}
	if !strings.Contains(content, "```vega-lite") {
		t.Error("missing vega-lite fence in scaffold")
	}
	if !strings.Contains(content, `## Notes`) {
		t.Error("missing ## Notes section in scaffold")
	}
	if !strings.Contains(content, `## Related`) {
		t.Error("missing ## Related section in scaffold")
	}
}

func TestNew_RefuseOverwrite(t *testing.T) {
	runner, _ := makeRunnerWithDataset(t, "sales")
	var buf bytes.Buffer
	// Create once.
	_ = runner.New(NewOptions{Slug: "revenue", DataSlug: "sales"}, &buf)
	// Try again — should fail.
	err := runner.New(NewOptions{Slug: "revenue", DataSlug: "sales"}, &buf)
	if err == nil {
		t.Fatal("expected error for overwrite attempt")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("want 'already exists' in error, got %q", err.Error())
	}
}

func TestNew_MissingDataFlag(t *testing.T) {
	runner, _ := makeRunnerWithDataset(t, "sales")
	var buf bytes.Buffer
	err := runner.New(NewOptions{Slug: "revenue", DataSlug: ""}, &buf)
	if err == nil {
		t.Fatal("expected error when --data is missing")
	}
	if !strings.Contains(err.Error(), "--data") {
		t.Errorf("want '--data' in error, got %q", err.Error())
	}
}

func TestNew_InvalidSlug(t *testing.T) {
	runner, _ := makeRunnerWithDataset(t, "sales")
	var buf bytes.Buffer
	err := runner.New(NewOptions{Slug: "Invalid_Slug", DataSlug: "sales"}, &buf)
	if err == nil {
		t.Fatal("expected error for invalid slug")
	}
	if !strings.Contains(err.Error(), "invalid slug") {
		t.Errorf("want 'invalid slug' in error, got %q", err.Error())
	}
}

func TestNew_DatasetNotFound(t *testing.T) {
	runner, _ := makeRunnerWithDataset(t, "sales")
	var buf bytes.Buffer
	err := runner.New(NewOptions{Slug: "revenue", DataSlug: "nonexistent"}, &buf)
	if err == nil {
		t.Fatal("expected error when dataset does not exist")
	}
	if !strings.Contains(err.Error(), "dataset not found") {
		t.Errorf("want 'dataset not found' in error, got %q", err.Error())
	}
}
