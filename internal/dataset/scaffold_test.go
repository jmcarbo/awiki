package dataset_test

import (
	"strings"
	"testing"

	"awiki/internal/dataset"
)

func TestWriteScaffold_NoFrom(t *testing.T) {
	got := string(dataset.WriteScaffold(dataset.ScaffoldInput{
		Slug:   "my-ds",
		Format: dataset.FormatCSV,
		Today:  "2026-04-29",
		Rows:   0,
	}))

	want := `---
title: "my-ds"
date: 2026-04-29
last_updated: 2026-04-29
type: dataset
tags: []
storage: inline
format: csv
rows: 0
sources: []
draft: false
---

# my-ds

<!-- one-paragraph description here -->

## Schema

<!-- describe each column here -->

## Data
` + "```csv\n```" + `

## Provenance

<!-- where these rows came from -->

## Related

## Sources
`
	if got != want {
		t.Errorf("scaffold mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestWriteScaffold_WithBody(t *testing.T) {
	body := []byte("name,age\nalice,30\n")
	got := string(dataset.WriteScaffold(dataset.ScaffoldInput{
		Slug:   "with-data",
		Format: dataset.FormatCSV,
		Today:  "2026-04-29",
		Rows:   1,
		Body:   body,
	}))

	if !strings.Contains(got, "name,age\nalice,30\n```") {
		t.Errorf("expected body in scaffold, got:\n%s", got)
	}
	if !strings.Contains(got, "rows: 1") {
		t.Errorf("expected rows: 1, got:\n%s", got)
	}
}
