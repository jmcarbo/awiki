package synth

import (
	"strings"
	"testing"
)

const fmFixture = `---
title: "My Page"
date: 2024-01-01
sources: []
last_generated:
---

Body content here.
`

func TestFmSetScalarHappyPath(t *testing.T) {
	got := FmSetScalar(fmFixture, "date", "2024-06-15")
	if !strings.Contains(got, "date: 2024-06-15") {
		t.Errorf("date not replaced: %q", got)
	}
	if strings.Contains(got, "date: 2024-01-01") {
		t.Errorf("old date still present: %q", got)
	}
}

func TestFmSetScalarMissingKeyNoOp(t *testing.T) {
	got := FmSetScalar(fmFixture, "nonexistent", "value")
	if got != fmFixture {
		t.Errorf("expected no-op for missing key, got %q", got)
	}
}

func TestFmSetScalarNoFrontmatter(t *testing.T) {
	body := "no frontmatter here\n"
	got := FmSetScalar(body, "title", "new")
	if got != body {
		t.Errorf("expected no-op for missing frontmatter, got %q", got)
	}
}

func TestFmSetSourcesEmptyList(t *testing.T) {
	got := FmSetSources(fmFixture, []string{})
	if !strings.Contains(got, "sources: []") {
		t.Errorf("expected empty sources list, got %q", got)
	}
}

func TestFmSetSourcesMultipleSlugs(t *testing.T) {
	got := FmSetSources(fmFixture, []string{"alpha", "beta"})
	want := `sources: ["[[alpha]]", "[[beta]]"]`
	if !strings.Contains(got, want) {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestFmSetSourcesSingleSlug(t *testing.T) {
	got := FmSetSources(fmFixture, []string{"foo"})
	want := `sources: ["[[foo]]"]`
	if !strings.Contains(got, want) {
		t.Errorf("expected %q, got %q", want, got)
	}
}
