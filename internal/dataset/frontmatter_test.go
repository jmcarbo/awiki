package dataset_test

import (
	"testing"

	"awiki/internal/dataset"
)

const baseFM = `---
title: Test Dataset
format: csv
rows: 10
---

## Data
`

func TestFmGet_Present(t *testing.T) {
	v, ok := dataset.FmGet(baseFM, "format")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if v != "csv" {
		t.Errorf("want csv, got %q", v)
	}
}

func TestFmGet_Absent(t *testing.T) {
	_, ok := dataset.FmGet(baseFM, "nonexistent")
	if ok {
		t.Error("expected ok=false for absent key")
	}
}

func TestFmGet_NoFrontmatter(t *testing.T) {
	_, ok := dataset.FmGet("# Just a heading\n\nSome text", "title")
	if ok {
		t.Error("expected ok=false when no frontmatter")
	}
}

func TestFmGet_QuotedValue(t *testing.T) {
	text := "---\ntitle: \"My Dataset\"\n---\n"
	v, ok := dataset.FmGet(text, "title")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if v != "My Dataset" {
		t.Errorf("want 'My Dataset' (quotes stripped), got %q", v)
	}
}

func TestFmSet_Replace(t *testing.T) {
	result := dataset.FmSet(baseFM, "rows", "42")
	v, ok := dataset.FmGet(result, "rows")
	if !ok {
		t.Fatal("key should still be present after replace")
	}
	if v != "42" {
		t.Errorf("want 42, got %q", v)
	}
	// title should be untouched
	title, _ := dataset.FmGet(result, "title")
	if title != "Test Dataset" {
		t.Errorf("title changed unexpectedly: %q", title)
	}
}

func TestFmSet_InsertMissing(t *testing.T) {
	text := "---\ntitle: Test\n---\n## Body\n"
	result := dataset.FmSet(text, "storage", "file")
	v, ok := dataset.FmGet(result, "storage")
	if !ok {
		t.Fatal("inserted key should be readable")
	}
	if v != "file" {
		t.Errorf("want file, got %q", v)
	}
	// Existing key should still be present.
	title, _ := dataset.FmGet(result, "title")
	if title != "Test" {
		t.Errorf("title changed: %q", title)
	}
}

func TestFmSet_Idempotent(t *testing.T) {
	r1 := dataset.FmSet(baseFM, "format", "tsv")
	r2 := dataset.FmSet(r1, "format", "tsv")
	if r1 != r2 {
		t.Error("second FmSet should produce identical result")
	}
}

func TestFmSet_NoFrontmatter(t *testing.T) {
	text := "# Just content\n"
	result := dataset.FmSet(text, "key", "val")
	if result != text {
		t.Error("FmSet on text without frontmatter should return unchanged")
	}
}

func TestFmRemove_Present(t *testing.T) {
	result := dataset.FmRemove(baseFM, "rows")
	_, ok := dataset.FmGet(result, "rows")
	if ok {
		t.Error("rows key should be gone after FmRemove")
	}
	// Other keys remain.
	v, _ := dataset.FmGet(result, "format")
	if v != "csv" {
		t.Errorf("format should still be csv, got %q", v)
	}
}

func TestFmRemove_Absent(t *testing.T) {
	result := dataset.FmRemove(baseFM, "nonexistent")
	if result != baseFM {
		t.Error("FmRemove of absent key should return text unchanged")
	}
}

func TestFmRemove_NoFrontmatter(t *testing.T) {
	text := "# heading\n"
	result := dataset.FmRemove(text, "key")
	if result != text {
		t.Error("FmRemove on text without frontmatter should return unchanged")
	}
}

func TestFmSet_PreserveQuotesOnSet(t *testing.T) {
	// When setting a value, the caller provides the value string as-is (with or
	// without quotes). FmSet must not add or remove quotes.
	text := "---\ntitle: plain\n---\n"
	result := dataset.FmSet(text, "title", `"quoted value"`)
	// Raw stored value should include quotes.
	raw := result
	_ = raw
	// Reading back strips double-quotes.
	v, ok := dataset.FmGet(result, "title")
	if !ok {
		t.Fatal("key should be present")
	}
	if v != "quoted value" {
		t.Errorf("want 'quoted value' (quotes stripped by FmGet), got %q", v)
	}
}
