package dataset_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"awiki/internal/dataset"
)

func makeNewRunner(t *testing.T) (*dataset.Runner, string) {
	t.Helper()
	root := t.TempDir()
	r := &dataset.Runner{
		RepoRoot:   root,
		ContentDir: filepath.Join(root, "content"),
		DataDir:    filepath.Join(root, "data"),
		Today:      "2026-04-29",
	}
	return r, root
}

func TestNew_Golden(t *testing.T) {
	r, root := makeNewRunner(t)
	var stdout bytes.Buffer
	if err := r.New(dataset.NewOptions{Slug: "my-ds", Format: "csv"}, &stdout); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	page := filepath.Join(root, "content", "datasets", "my-ds.md")
	data, err := os.ReadFile(page)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "title: \"my-ds\"") {
		t.Errorf("expected title in page, got:\n%s", got)
	}
	if !strings.Contains(got, "storage: inline") {
		t.Errorf("expected storage: inline, got:\n%s", got)
	}
	if !strings.Contains(got, "format: csv") {
		t.Errorf("expected format: csv, got:\n%s", got)
	}
	if !strings.Contains(got, "## Provenance") {
		t.Errorf("expected ## Provenance section, got:\n%s", got)
	}
	if !strings.Contains(stdout.String(), "DATASET|created") {
		t.Errorf("expected DATASET|created in stdout, got: %s", stdout.String())
	}
}

func TestNew_FromFile(t *testing.T) {
	r, root := makeNewRunner(t)
	srcFile := filepath.Join(root, "source.csv")
	if err := os.WriteFile(srcFile, []byte("name,age\nalice,30\nbob,25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	if err := r.New(dataset.NewOptions{Slug: "from-ds", Format: "csv", From: srcFile}, &stdout); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	page := filepath.Join(root, "content", "datasets", "from-ds.md")
	data, err := os.ReadFile(page)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "rows: 2") {
		t.Errorf("expected rows: 2, got:\n%s", got)
	}
	if !strings.Contains(got, "name,age\nalice,30\nbob,25") {
		t.Errorf("expected CSV body embedded, got:\n%s", got)
	}
}

func TestNew_RefuseOverwrite(t *testing.T) {
	r, root := makeNewRunner(t)
	dsDir := filepath.Join(root, "content", "datasets")
	if err := os.MkdirAll(dsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(dsDir, "exists.md")
	if err := os.WriteFile(existing, []byte("---\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	err := r.New(dataset.NewOptions{Slug: "exists", Format: "csv"}, &stdout)
	if err == nil {
		t.Fatal("expected error for overwrite attempt")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNew_InvalidSlug(t *testing.T) {
	r, _ := makeNewRunner(t)
	var stdout bytes.Buffer
	err := r.New(dataset.NewOptions{Slug: "Invalid_Slug", Format: "csv"}, &stdout)
	if err == nil {
		t.Fatal("expected error for invalid slug")
	}
	if !strings.Contains(err.Error(), "invalid slug") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNew_UnsupportedFormat(t *testing.T) {
	r, _ := makeNewRunner(t)
	var stdout bytes.Buffer
	err := r.New(dataset.NewOptions{Slug: "my-ds", Format: "xlsx"}, &stdout)
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
	if !strings.Contains(err.Error(), "unsupported format") {
		t.Errorf("unexpected error: %v", err)
	}
}
