package dataset_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"awiki/internal/dataset"
)

func makeRunnerWithThresholds(t *testing.T, maxRows, maxBytes int) (*dataset.Runner, string) {
	t.Helper()
	root := t.TempDir()
	dsDir := filepath.Join(root, "content", "datasets")
	dataDir := filepath.Join(root, "data")
	awikiDir := filepath.Join(root, ".awiki")
	if err := os.MkdirAll(dsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(awikiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write config with small thresholds.
	cfg := filepath.Join(awikiDir, "config")
	content := ""
	if maxRows >= 0 {
		content += "AWIKI_DATASET_INLINE_MAX_ROWS=" + itoa(maxRows) + "\n"
	}
	if maxBytes >= 0 {
		content += "AWIKI_DATASET_INLINE_MAX_BYTES=" + itoa(maxBytes) + "\n"
	}
	if err := os.WriteFile(cfg, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &dataset.Runner{
		RepoRoot:   root,
		ContentDir: filepath.Join(root, "content"),
		DataDir:    dataDir,
	}
	return r, root
}

func itoa(n int) string {
	return strings.TrimSpace(strings.NewReplacer().Replace(
		// use a simple approach
		string(rune('0'+n%10)),
	))
}

func writeDatasetPageCompact(t *testing.T, root, slug, content string) string {
	t.Helper()
	path := filepath.Join(root, "content", "datasets", slug+".md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCompact_InlineToFile(t *testing.T) {
	// Use small thresholds so inline gets promoted to file.
	root := t.TempDir()
	dsDir := filepath.Join(root, "content", "datasets")
	dataDir := filepath.Join(root, "data")
	awikiDir := filepath.Join(root, ".awiki")
	for _, d := range []string{dsDir, dataDir, awikiDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// maxRows=1, maxBytes=100000 — 2 data rows exceeds 1
	cfg := filepath.Join(awikiDir, "config")
	if err := os.WriteFile(cfg, []byte("AWIKI_DATASET_INLINE_MAX_ROWS=1\nAWIKI_DATASET_INLINE_MAX_BYTES=100000\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &dataset.Runner{
		RepoRoot:   root,
		ContentDir: filepath.Join(root, "content"),
		DataDir:    dataDir,
	}

	page := `---
title: "myds"
storage: inline
format: csv
rows: 0
---

## Data
` + "```csv\nname,age\nalice,30\nbob,25\n```" + `

## Provenance
`
	writeDatasetPageCompact(t, root, "myds", page)

	var stdout bytes.Buffer
	if err := r.Compact("myds", &stdout); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "inline -> file") {
		t.Errorf("expected inline->file message, got: %s", out)
	}
	// Data file should exist.
	target := filepath.Join(dataDir, "myds.csv")
	if _, err := os.Stat(target); err != nil {
		t.Errorf("expected data file %s, got: %v", target, err)
	}
	// Page should reference file storage.
	updated, _ := os.ReadFile(filepath.Join(root, "content", "datasets", "myds.md"))
	if !strings.Contains(string(updated), "storage: file") {
		t.Error("expected storage: file in updated page")
	}
}

func TestCompact_FileToInline(t *testing.T) {
	root := t.TempDir()
	dsDir := filepath.Join(root, "content", "datasets")
	dataDir := filepath.Join(root, "data")
	awikiDir := filepath.Join(root, ".awiki")
	for _, d := range []string{dsDir, dataDir, awikiDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// High thresholds so file gets pulled back inline.
	cfg := filepath.Join(awikiDir, "config")
	if err := os.WriteFile(cfg, []byte("AWIKI_DATASET_INLINE_MAX_ROWS=500\nAWIKI_DATASET_INLINE_MAX_BYTES=51200\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	csvData := "name,age\nalice,30\n"
	csvPath := filepath.Join(dataDir, "inline-me.csv")
	if err := os.WriteFile(csvPath, []byte(csvData), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &dataset.Runner{
		RepoRoot:   root,
		ContentDir: filepath.Join(root, "content"),
		DataDir:    dataDir,
	}

	page := `---
title: "inline-me"
storage: file
format: csv
data_path: data/inline-me.csv
rows: 1
---

## Data

## Provenance

Some notes here.
`
	writeDatasetPageCompact(t, root, "inline-me", page)

	var stdout bytes.Buffer
	if err := r.Compact("inline-me", &stdout); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "file -> inline") {
		t.Errorf("expected file->inline message, got: %s", out)
	}
	// Page should now have inline storage and no data_path.
	updated, _ := os.ReadFile(filepath.Join(root, "content", "datasets", "inline-me.md"))
	updatedStr := string(updated)
	if !strings.Contains(updatedStr, "storage: inline") {
		t.Error("expected storage: inline in updated page")
	}
	if strings.Contains(updatedStr, "data_path") {
		t.Error("expected data_path to be removed")
	}
	// Data file should be removed.
	if _, err := os.Stat(csvPath); !os.IsNotExist(err) {
		t.Error("expected data file to be removed after inline")
	}
}

func TestCompact_AlreadyCompact(t *testing.T) {
	root := t.TempDir()
	dsDir := filepath.Join(root, "content", "datasets")
	awikiDir := filepath.Join(root, ".awiki")
	for _, d := range []string{dsDir, awikiDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cfg := filepath.Join(awikiDir, "config")
	if err := os.WriteFile(cfg, []byte("AWIKI_DATASET_INLINE_MAX_ROWS=500\nAWIKI_DATASET_INLINE_MAX_BYTES=51200\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &dataset.Runner{
		RepoRoot:   root,
		ContentDir: filepath.Join(root, "content"),
		DataDir:    filepath.Join(root, "data"),
	}

	page := `---
title: "compact"
storage: inline
format: csv
rows: 1
---

## Data
` + "```csv\nname,age\nalice,30\n```" + `

## Provenance
`
	writeDatasetPageCompact(t, root, "compact", page)

	var stdout bytes.Buffer
	if err := r.Compact("compact", &stdout); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "already compact") {
		t.Errorf("expected 'already compact', got: %s", stdout.String())
	}
}

func TestCompact_FileOverThresholdError(t *testing.T) {
	root := t.TempDir()
	dsDir := filepath.Join(root, "content", "datasets")
	dataDir := filepath.Join(root, "data")
	awikiDir := filepath.Join(root, ".awiki")
	for _, d := range []string{dsDir, dataDir, awikiDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Small threshold so the file data is over.
	cfg := filepath.Join(awikiDir, "config")
	if err := os.WriteFile(cfg, []byte("AWIKI_DATASET_INLINE_MAX_ROWS=1\nAWIKI_DATASET_INLINE_MAX_BYTES=5\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	csvData := "name,age\nalice,30\nbob,25\ncharlie,40\n"
	csvPath := filepath.Join(dataDir, "big.csv")
	if err := os.WriteFile(csvPath, []byte(csvData), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &dataset.Runner{
		RepoRoot:   root,
		ContentDir: filepath.Join(root, "content"),
		DataDir:    dataDir,
	}

	page := `---
title: "big"
storage: file
format: csv
data_path: data/big.csv
rows: 3
---

## Data

## Provenance
`
	writeDatasetPageCompact(t, root, "big", page)

	var stdout bytes.Buffer
	err := r.Compact("big", &stdout)
	if err == nil {
		t.Fatal("expected error when file is over threshold")
	}
	if !strings.Contains(err.Error(), "over threshold") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCompact_FileToInlineMissingProvenance(t *testing.T) {
	root := t.TempDir()
	dsDir := filepath.Join(root, "content", "datasets")
	dataDir := filepath.Join(root, "data")
	awikiDir := filepath.Join(root, ".awiki")
	for _, d := range []string{dsDir, dataDir, awikiDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// High thresholds.
	cfg := filepath.Join(awikiDir, "config")
	if err := os.WriteFile(cfg, []byte("AWIKI_DATASET_INLINE_MAX_ROWS=500\nAWIKI_DATASET_INLINE_MAX_BYTES=51200\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	csvData := "name\nalice\n"
	csvPath := filepath.Join(dataDir, "noprov.csv")
	if err := os.WriteFile(csvPath, []byte(csvData), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &dataset.Runner{
		RepoRoot:   root,
		ContentDir: filepath.Join(root, "content"),
		DataDir:    dataDir,
	}

	// Page without ## Provenance.
	page := `---
title: "noprov"
storage: file
format: csv
data_path: data/noprov.csv
rows: 1
---

## Data

`
	writeDatasetPageCompact(t, root, "noprov", page)

	var stdout bytes.Buffer
	err := r.Compact("noprov", &stdout)
	if err == nil {
		t.Fatal("expected error for missing ## Provenance")
	}
	if !strings.Contains(err.Error(), "Provenance") {
		t.Errorf("unexpected error: %v", err)
	}
}
