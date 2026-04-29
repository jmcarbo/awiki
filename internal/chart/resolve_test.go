package chart

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeRepo creates a temporary repository root with a datasets directory.
// Each entry in datasets maps slug → file content.
func makeRepo(t *testing.T, datasets map[string]string) string {
	t.Helper()
	root := t.TempDir()
	dsDir := filepath.Join(root, "content", "datasets")
	if err := os.MkdirAll(dsDir, 0755); err != nil {
		t.Fatal(err)
	}
	for slug, content := range datasets {
		if err := os.WriteFile(filepath.Join(dsDir, slug+".md"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const csvDataset = `---
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

const fileDataset = `---
title: "expenses"
type: dataset
storage: file
format: csv
data_path: "data/expenses.csv"
---

## Data
`

func TestResolveSpec_InlineCSV(t *testing.T) {
	root := makeRepo(t, map[string]string{"sales": csvDataset})
	spec := map[string]any{
		"mark": "bar",
		"data": map[string]any{"name": "[[sales]]"},
	}
	resolved, err := ResolveSpec(root, "/", "test.md", spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, ok := resolved["data"].(map[string]any)
	if !ok {
		t.Fatalf("data is not a map: %T", resolved["data"])
	}
	values, ok := data["values"]
	if !ok {
		t.Fatalf("expected 'values' key in resolved data")
	}
	rows, ok := values.([]map[string]any)
	if !ok {
		t.Fatalf("values is not []map[string]any: %T", values)
	}
	if len(rows) != 2 {
		t.Errorf("want 2 rows, got %d", len(rows))
	}
	if rows[0]["month"] != "Jan" {
		t.Errorf("want month=Jan, got %v", rows[0]["month"])
	}
}

func TestResolveSpec_FileStorage(t *testing.T) {
	root := makeRepo(t, map[string]string{"expenses": fileDataset})
	spec := map[string]any{
		"mark": "bar",
		"data": map[string]any{"name": "[[expenses]]"},
	}
	resolved, err := ResolveSpec(root, "/", "test.md", spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, ok := resolved["data"].(map[string]any)
	if !ok {
		t.Fatalf("data is not a map: %T", resolved["data"])
	}
	url, ok := data["url"].(string)
	if !ok {
		t.Fatal("expected 'url' key")
	}
	if !strings.Contains(url, "expenses.csv") {
		t.Errorf("url should contain expenses.csv, got %q", url)
	}
	fmtBlock, ok := data["format"].(map[string]any)
	if !ok {
		t.Fatal("expected 'format' key")
	}
	if fmtBlock["type"] != "csv" {
		t.Errorf("want format type=csv, got %v", fmtBlock["type"])
	}
}

func TestResolveSpec_MissingDataset(t *testing.T) {
	root := makeRepo(t, nil)
	spec := map[string]any{
		"mark": "bar",
		"data": map[string]any{"name": "[[missing]]"},
	}
	_, err := ResolveSpec(root, "/", "test.md", spec)
	if err == nil {
		t.Fatal("expected error for missing dataset")
	}
	if !strings.Contains(err.Error(), "not_found") {
		t.Errorf("want 'not_found' in error, got %q", err.Error())
	}
}

func TestResolveSpec_NonDatasetType(t *testing.T) {
	root := makeRepo(t, map[string]string{"notadataset": `---
title: "not a dataset"
type: note
---
`})
	spec := map[string]any{
		"data": map[string]any{"name": "[[notadataset]]"},
	}
	_, err := ResolveSpec(root, "/", "test.md", spec)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not_a_dataset") {
		t.Errorf("want 'not_a_dataset' in error, got %q", err.Error())
	}
}

func TestResolveSpec_PassThroughNonRefData(t *testing.T) {
	root := makeRepo(t, nil)
	spec := map[string]any{
		"mark": "bar",
		"data": map[string]any{"values": []any{map[string]any{"x": 1}}},
	}
	resolved, err := ResolveSpec(root, "/", "test.md", spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should pass through unchanged.
	data := resolved["data"].(map[string]any)
	if _, ok := data["values"]; !ok {
		t.Error("expected values to pass through")
	}
}

func TestResolveSpec_JSONRoundTrip(t *testing.T) {
	root := makeRepo(t, map[string]string{"sales": csvDataset})
	spec := map[string]any{
		"mark": "bar",
		"data": map[string]any{"name": "[[sales]]"},
	}
	resolved, err := ResolveSpec(root, "/", "test.md", spec)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(resolved)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	if !strings.Contains(string(b), "revenue") {
		t.Errorf("resolved JSON should contain 'revenue': %s", string(b))
	}
}
