package query

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNew_ScaffoldGolden(t *testing.T) {
	tmp := t.TempDir()
	runner := &Runner{
		RepoRoot:   tmp,
		ContentDir: filepath.Join(tmp, "content"),
		DataDir:    filepath.Join(tmp, "data"),
		Today:      "2024-01-01",
	}
	os.MkdirAll(runner.QueryDir(), 0755)

	var buf bytes.Buffer
	err := runner.New(NewOptions{Slug: "my-query", OutSlug: "my-dataset"}, &buf)
	if err != nil {
		t.Fatal(err)
	}

	// Check output message.
	if !strings.Contains(buf.String(), "QUERY|NEW|") {
		t.Errorf("expected QUERY|NEW| output, got: %q", buf.String())
	}

	// Check the file was created.
	page := filepath.Join(runner.QueryDir(), "my-query.md")
	content, err := os.ReadFile(page)
	if err != nil {
		t.Fatalf("expected file at %s", page)
	}
	text := string(content)

	// Check frontmatter keys.
	if !strings.Contains(text, `title: "my-query"`) {
		t.Error("missing title in frontmatter")
	}
	if !strings.Contains(text, "type: query") {
		t.Error("missing type: query in frontmatter")
	}
	if !strings.Contains(text, "out: my-dataset") {
		t.Error("missing out: my-dataset in frontmatter")
	}
	if !strings.Contains(text, "date: 2024-01-01") {
		t.Error("missing date in frontmatter")
	}
	// Check SQL stub.
	if !strings.Contains(text, "SELECT 1 AS placeholder") {
		t.Error("missing SQL placeholder")
	}
	if !strings.Contains(text, "ORDER BY 1") {
		t.Error("missing ORDER BY in SQL stub")
	}
}

func TestNew_RefuseOverwrite(t *testing.T) {
	tmp := t.TempDir()
	runner := &Runner{
		RepoRoot:   tmp,
		ContentDir: filepath.Join(tmp, "content"),
		DataDir:    filepath.Join(tmp, "data"),
		Today:      "2024-01-01",
	}
	os.MkdirAll(runner.QueryDir(), 0755)

	// Create the page first.
	page := filepath.Join(runner.QueryDir(), "existing.md")
	os.WriteFile(page, []byte("existing"), 0644)

	var buf bytes.Buffer
	err := runner.New(NewOptions{Slug: "existing", OutSlug: "foo"}, &buf)
	if err == nil {
		t.Error("expected error when page already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should mention already exists: %v", err)
	}
}

func TestNew_BadSlug(t *testing.T) {
	tmp := t.TempDir()
	runner := &Runner{
		RepoRoot:   tmp,
		ContentDir: filepath.Join(tmp, "content"),
		Today:      "2024-01-01",
	}
	var buf bytes.Buffer
	err := runner.New(NewOptions{Slug: "Bad Slug!", OutSlug: "foo"}, &buf)
	if err == nil {
		t.Error("expected error for bad slug")
	}
}

func TestNew_MissingOut(t *testing.T) {
	tmp := t.TempDir()
	runner := &Runner{
		RepoRoot:   tmp,
		ContentDir: filepath.Join(tmp, "content"),
		Today:      "2024-01-01",
	}
	var buf bytes.Buffer
	err := runner.New(NewOptions{Slug: "my-query", OutSlug: ""}, &buf)
	if err == nil {
		t.Error("expected error when --out is missing")
	}
}
