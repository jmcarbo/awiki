package query

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeQueryPage(dir, slug, outSlug, sql string) {
	content := "---\ntitle: \"" + slug + "\"\ntype: query\nout: " + outSlug + "\nsql_hash: \"\"\n---\n\n# " + slug + "\n\n## SQL\n\n```sql\n" + sql + "\n```\n\n## Notes\n"
	os.WriteFile(filepath.Join(dir, slug+".md"), []byte(content), 0644)
}

func TestRenderOne_Basic(t *testing.T) {
	tmp := t.TempDir()
	contentDir := filepath.Join(tmp, "content")
	queryDir := filepath.Join(contentDir, "queries")
	datasetDir := filepath.Join(contentDir, "datasets")
	os.MkdirAll(queryDir, 0755)
	os.MkdirAll(datasetDir, 0755)

	makeQueryPage(queryDir, "my-query", "my-result", "SELECT 1 AS n ORDER BY n")

	runner := &Runner{
		RepoRoot:   tmp,
		ContentDir: contentDir,
		DataDir:    filepath.Join(tmp, "data"),
		DuckDB:     mockDuckDB{output: "n\n1\n", code: 0},
		Today:      "2024-01-01",
	}

	var buf bytes.Buffer
	err := runner.RenderOne(context.Background(), "my-query", &buf)
	if err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "QUERY|MATERIALIZED") && !strings.Contains(out, "QUERY|RENDER") {
		t.Errorf("expected materialization output, got: %q", out)
	}

	// Dataset page should be created.
	target := filepath.Join(datasetDir, "my-result.md")
	if _, err := os.Stat(target); os.IsNotExist(err) {
		t.Errorf("expected dataset page at %s", target)
	}
}

func TestRenderOne_MissingPage(t *testing.T) {
	tmp := t.TempDir()
	runner := &Runner{
		RepoRoot:   tmp,
		ContentDir: filepath.Join(tmp, "content"),
		DataDir:    filepath.Join(tmp, "data"),
		DuckDB:     mockDuckDB{},
	}
	var buf bytes.Buffer
	err := runner.RenderOne(context.Background(), "nonexistent", &buf)
	if err == nil {
		t.Error("expected error for missing page")
	}
}

func TestRender_WalksQueryDir(t *testing.T) {
	tmp := t.TempDir()
	contentDir := filepath.Join(tmp, "content")
	queryDir := filepath.Join(contentDir, "queries")
	datasetDir := filepath.Join(contentDir, "datasets")
	os.MkdirAll(queryDir, 0755)
	os.MkdirAll(datasetDir, 0755)

	makeQueryPage(queryDir, "q1", "result1", "SELECT 1 AS n ORDER BY n")
	makeQueryPage(queryDir, "q2", "result2", "SELECT 2 AS n ORDER BY n")

	runner := &Runner{
		RepoRoot:   tmp,
		ContentDir: contentDir,
		DataDir:    filepath.Join(tmp, "data"),
		DuckDB:     mockDuckDB{output: "n\n1\n", code: 0},
		Today:      "2024-01-01",
	}

	var buf bytes.Buffer
	err := runner.Render(context.Background(), &buf)
	if err != nil {
		t.Fatal(err)
	}

	// Both pages should be rendered.
	for _, slug := range []string{"result1", "result2"} {
		target := filepath.Join(datasetDir, slug+".md")
		if _, err := os.Stat(target); os.IsNotExist(err) {
			t.Errorf("expected dataset page at %s", target)
		}
	}
}

func TestRender_EmptyDir(t *testing.T) {
	tmp := t.TempDir()
	runner := &Runner{
		RepoRoot:   tmp,
		ContentDir: filepath.Join(tmp, "content"),
		DataDir:    filepath.Join(tmp, "data"),
		DuckDB:     mockDuckDB{},
	}
	var buf bytes.Buffer
	// Should not error when query dir doesn't exist.
	if err := runner.Render(context.Background(), &buf); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
