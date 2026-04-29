package dataset_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"awiki/internal/dataset"
)

func makeRunner(t *testing.T) (*dataset.Runner, string) {
	t.Helper()
	root := t.TempDir()
	dsDir := filepath.Join(root, "content", "datasets")
	if err := os.MkdirAll(dsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	r := &dataset.Runner{
		RepoRoot:   root,
		ContentDir: filepath.Join(root, "content"),
		DataDir:    filepath.Join(root, "data"),
	}
	return r, root
}

func writeDatasetPage(t *testing.T, root, slug, content string) string {
	t.Helper()
	path := filepath.Join(root, "content", "datasets", slug+".md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestValidate_CSVInlineOK(t *testing.T) {
	r, root := makeRunner(t)
	page := `---
title: "test"
storage: inline
format: csv
rows: 0
---

## Data
` + "```csv\nname,age\nalice,30\nbob,25\n```" + `

## Provenance
`
	writeDatasetPage(t, root, "test", page)

	var stdout, stderr bytes.Buffer
	if err := r.Validate("test", &stdout, &stderr); err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "DATASET|validated test (rows=2)") {
		t.Errorf("unexpected stdout: %q", out)
	}
}

func TestValidate_CSVInlineSchemaFail(t *testing.T) {
	r, root := makeRunner(t)
	page := `---
title: "test"
storage: inline
format: csv
rows: 0
columns: [{name: age, type: integer}]
---

## Data
` + "```csv\nname,age\nalice,notanumber\n```" + `

## Provenance
`
	writeDatasetPage(t, root, "test-schema", page)

	var stdout, stderr bytes.Buffer
	err := r.Validate("test-schema", &stdout, &stderr)
	if err == nil {
		t.Fatal("expected schema validation error")
	}
	if !strings.Contains(err.Error(), "schema validation failed") {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr.String(), "DATASET|ERROR|") {
		t.Errorf("expected DATASET|ERROR| in stderr, got: %s", stderr.String())
	}
}

func TestValidate_JSONFileStorage(t *testing.T) {
	r, root := makeRunner(t)
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jsonData := `[{"name":"alice","score":10},{"name":"bob","score":20}]`
	jsonPath := filepath.Join(dataDir, "test-json.json")
	if err := os.WriteFile(jsonPath, []byte(jsonData), 0o644); err != nil {
		t.Fatal(err)
	}

	page := `---
title: "test-json"
storage: file
format: json
data_path: data/test-json.json
rows: 0
---

## Data

## Provenance
`
	writeDatasetPage(t, root, "test-json", page)

	var stdout, stderr bytes.Buffer
	if err := r.Validate("test-json", &stdout, &stderr); err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "DATASET|validated test-json (rows=2)") {
		t.Errorf("unexpected stdout: %q", out)
	}
}

func TestValidate_MissingPage(t *testing.T) {
	r, _ := makeRunner(t)
	var stdout, stderr bytes.Buffer
	err := r.Validate("nonexistent", &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for missing page")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_BadDataPath(t *testing.T) {
	r, root := makeRunner(t)
	page := `---
title: "test-bad"
storage: file
format: csv
data_path: data/missing.csv
rows: 0
---

## Data

## Provenance
`
	writeDatasetPage(t, root, "test-bad", page)

	var stdout, stderr bytes.Buffer
	err := r.Validate("test-bad", &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for missing data_path")
	}
	if !strings.Contains(err.Error(), "not found or unreadable") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_RefreshesRowCount(t *testing.T) {
	r, root := makeRunner(t)
	page := `---
title: "test-refresh"
storage: inline
format: csv
rows: 999
---

## Data
` + "```csv\nname,age\nalice,30\n```" + `

## Provenance
`
	path := writeDatasetPage(t, root, "test-refresh", page)

	var stdout, stderr bytes.Buffer
	if err := r.Validate("test-refresh", &stdout, &stderr); err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, stderr.String())
	}

	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), "rows: 1") {
		t.Errorf("expected rows: 1 in updated page, got:\n%s", string(updated))
	}
}
