package dataset_test

import (
	"strings"
	"testing"

	"awiki/internal/dataset"
)

const docWithFence = `---
title: Test
---

## Data

` + "```csv\n" + `name,age
Alice,30
Bob,25
` + "```" + `

## Notes

Some notes here.
`

const docNoFence = `---
title: Test
---

## Data

## Notes
`

const docNoDataSection = `---
title: Test
---

## Notes

Some notes.
`

func TestExtractDataFence_Present(t *testing.T) {
	content, format, ok := dataset.ExtractDataFence(docWithFence)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if format != dataset.FormatCSV {
		t.Errorf("want csv format, got %q", format)
	}
	if !strings.Contains(string(content), "Alice") {
		t.Errorf("content should contain Alice, got %q", string(content))
	}
}

func TestExtractDataFence_Absent(t *testing.T) {
	_, _, ok := dataset.ExtractDataFence(docNoFence)
	if ok {
		t.Error("expected ok=false when no fence")
	}
}

func TestExtractDataFence_NoDataSection(t *testing.T) {
	_, _, ok := dataset.ExtractDataFence(docNoDataSection)
	if ok {
		t.Error("expected ok=false when no ## Data heading")
	}
}

func TestExtractDataFence_EmptyFence(t *testing.T) {
	doc := "## Data\n\n```csv\n```\n"
	content, format, ok := dataset.ExtractDataFence(doc)
	if !ok {
		t.Fatal("expected ok=true for empty fence")
	}
	if format != dataset.FormatCSV {
		t.Errorf("want csv, got %q", format)
	}
	if len(content) != 0 {
		t.Errorf("expected empty content, got %q", string(content))
	}
}

func TestReplaceDataFence_Existing(t *testing.T) {
	newContent := []byte("x,y\n1,2\n")
	result, err := dataset.ReplaceDataFence(docWithFence, dataset.FormatCSV, newContent)
	if err != nil {
		t.Fatal(err)
	}
	got, _, ok := dataset.ExtractDataFence(result)
	if !ok {
		t.Fatal("fence should still be present after replace")
	}
	if !strings.Contains(string(got), "x,y") {
		t.Errorf("replaced content should contain new data, got %q", string(got))
	}
	if strings.Contains(string(got), "Alice") {
		t.Error("old content should be gone")
	}
}

func TestReplaceDataFence_NoFence(t *testing.T) {
	// ## Data exists but no fence — should insert one.
	result, err := dataset.ReplaceDataFence(docNoFence, dataset.FormatCSV, []byte("a,b\n1,2\n"))
	if err != nil {
		t.Fatal(err)
	}
	got, _, ok := dataset.ExtractDataFence(result)
	if !ok {
		t.Fatal("fence should be inserted")
	}
	if !strings.Contains(string(got), "a,b") {
		t.Errorf("inserted content not found: %q", string(got))
	}
}

func TestReplaceDataFence_NoDataSection(t *testing.T) {
	_, err := dataset.ReplaceDataFence(docNoDataSection, dataset.FormatCSV, []byte("a,b\n"))
	if err == nil {
		t.Error("expected error when ## Data section missing")
	}
}

func TestRemoveDataFence(t *testing.T) {
	result := dataset.RemoveDataFence(docWithFence)
	_, _, ok := dataset.ExtractDataFence(result)
	if ok {
		t.Error("fence should be gone after RemoveDataFence")
	}
	if !strings.Contains(result, "## Data") {
		t.Error("## Data heading should remain")
	}
}

func TestRemoveDataFence_NoFence(t *testing.T) {
	// Should return unchanged when no fence present.
	result := dataset.RemoveDataFence(docNoFence)
	if result != docNoFence {
		t.Error("RemoveDataFence with no fence should return text unchanged")
	}
}

func TestRemoveDataFence_NoDataSection(t *testing.T) {
	result := dataset.RemoveDataFence(docNoDataSection)
	if result != docNoDataSection {
		t.Error("RemoveDataFence with no ## Data should return text unchanged")
	}
}
