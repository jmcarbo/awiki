package query

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFenceRender_SingleFence(t *testing.T) {
	tmp := t.TempDir()
	contentDir := filepath.Join(tmp, "content")
	os.MkdirAll(contentDir, 0755)

	pageContent := "# Test\n\n```awiki-query out=my-fence\nSELECT 1 AS n\n```\n\nSome text.\n"
	pagePath := filepath.Join(contentDir, "test.md")
	os.WriteFile(pagePath, []byte(pageContent), 0644)

	runner := &Runner{
		RepoRoot:   tmp,
		ContentDir: contentDir,
		DataDir:    filepath.Join(tmp, "data"),
		DuckDB:     mockDuckDB{output: `[{"n":1}]`, code: 0},
		Today:      "2024-01-01",
	}

	var buf bytes.Buffer
	err := runner.FenceRender(context.Background(), &buf)
	if err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "QUERY|FENCE-RENDER") {
		t.Errorf("expected FENCE-RENDER output, got: %q", out)
	}

	// Check managed region was written.
	updatedBytes, _ := os.ReadFile(pagePath)
	updated := string(updatedBytes)
	if !strings.Contains(updated, "<!-- BEGIN awiki-query:") {
		t.Errorf("expected managed region in updated page: %q", updated)
	}
}

func TestFenceRender_MultiFence(t *testing.T) {
	tmp := t.TempDir()
	contentDir := filepath.Join(tmp, "content")
	os.MkdirAll(contentDir, 0755)

	pageContent := "# Test\n\n```awiki-query out=fence1\nSELECT 1\n```\n\nMiddle.\n\n```awiki-query out=fence2\nSELECT 2\n```\n"
	pagePath := filepath.Join(contentDir, "multi.md")
	os.WriteFile(pagePath, []byte(pageContent), 0644)

	runner := &Runner{
		RepoRoot:   tmp,
		ContentDir: contentDir,
		DataDir:    filepath.Join(tmp, "data"),
		DuckDB:     mockDuckDB{output: `[{"n":1}]`, code: 0},
		Today:      "2024-01-01",
	}

	var buf bytes.Buffer
	err := runner.FenceRender(context.Background(), &buf)
	if err != nil {
		t.Fatal(err)
	}

	// Both fences should appear in output.
	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	count := 0
	for _, l := range lines {
		if strings.Contains(l, "QUERY|FENCE-RENDER") {
			count++
		}
	}
	if count != 2 {
		t.Errorf("expected 2 FENCE-RENDER lines, got %d: %q", count, out)
	}

	// .queries.json sidecar should be written.
	sidecar := filepath.Join(contentDir, "multi.queries.json")
	if _, err := os.Stat(sidecar); os.IsNotExist(err) {
		t.Errorf("expected .queries.json sidecar at %s", sidecar)
	}
}

func TestFenceRender_NoFences(t *testing.T) {
	tmp := t.TempDir()
	contentDir := filepath.Join(tmp, "content")
	os.MkdirAll(contentDir, 0755)

	os.WriteFile(filepath.Join(contentDir, "plain.md"), []byte("# No fences here\n"), 0644)

	runner := &Runner{
		RepoRoot:   tmp,
		ContentDir: contentDir,
		DataDir:    filepath.Join(tmp, "data"),
		DuckDB:     mockDuckDB{},
	}

	var buf bytes.Buffer
	if err := runner.FenceRender(context.Background(), &buf); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buf.Len() > 0 {
		t.Errorf("expected no output for page with no fences, got: %q", buf.String())
	}
}
