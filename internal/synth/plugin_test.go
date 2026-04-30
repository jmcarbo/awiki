package synth

import (
	"os"
	"path/filepath"
	"testing"
)

const samplePlugin = `---
name: explainer
description: One-paragraph topic explainer.
output_type: synth
output_subtype: explainer
min_sources: 2
max_sources: 8
max_evidence_total_words: 500
required_sections:
  - Summary
  - Sources
post_hook: awiki synth-mindmap-validate
render: markdown
version: 1
---
Body line 1
Body line 2
`

func TestLoadPluginParsesBlockList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "explainer.md")
	if err := os.WriteFile(path, []byte(samplePlugin), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadPlugin(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "explainer" {
		t.Fatalf("name: %q", got.Name)
	}
	if got.MinSources != 2 || got.MaxSources != 8 {
		t.Fatalf("min/max: %d/%d", got.MinSources, got.MaxSources)
	}
	if len(got.RequiredSections) != 2 || got.RequiredSections[0] != "Summary" {
		t.Fatalf("required: %v", got.RequiredSections)
	}
	if got.PostHook != "awiki synth-mindmap-validate" {
		t.Fatalf("post_hook: %q", got.PostHook)
	}
	if got.Body != "Body line 1\nBody line 2\n" {
		t.Fatalf("body: %q", got.Body)
	}
}

func TestLoadPluginInlineList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "p.md")
	body := "---\nname: p\nrequired_sections: [Sum, Notes]\nversion: 1\n---\nx\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadPlugin(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.RequiredSections) != 2 || got.RequiredSections[1] != "Notes" {
		t.Fatalf("required: %v", got.RequiredSections)
	}
}

func TestLoadPluginRequiresName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "noname.md")
	body := "---\nversion: 1\n---\n\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPlugin(path); err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestLoadPluginRequiresFilenameMatchesName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "actual.md")
	body := "---\nname: different\nversion: 1\n---\n\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPlugin(path); err == nil {
		t.Fatal("expected error: name must match filename")
	}
}
