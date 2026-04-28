package wiki

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParsePageFrontmatterBodyAndInlineLists(t *testing.T) {
	contentDir := t.TempDir()
	path := writePage(t, contentDir, "entities/foo.md", `---
title: "Foo"
date: 2026-04-27
last_updated: 2026-04-28
type: entity
tags: [private, test]
aliases: [Foo, F.]
sources: ["[[s-foo]]"]
draft: false
---
Lead paragraph with [[bar]] and [[baz|Baz Display]].
`)

	page, err := ParsePage(path, contentDir)
	if err != nil {
		t.Fatalf("ParsePage() error = %v", err)
	}

	if page.Path != path {
		t.Fatalf("Path = %q, want %q", page.Path, path)
	}
	if page.RelPath != "entities/foo.md" {
		t.Fatalf("RelPath = %q, want entities/foo.md", page.RelPath)
	}
	if page.Slug != "foo" {
		t.Fatalf("Slug = %q, want foo", page.Slug)
	}
	if page.Title != "Foo" || page.Type != "entity" {
		t.Fatalf("Title/Type = %q/%q, want Foo/entity", page.Title, page.Type)
	}
	if page.Date != "2026-04-27" || page.LastUpdated != "2026-04-28" {
		t.Fatalf("Date/LastUpdated = %q/%q, want 2026-04-27/2026-04-28", page.Date, page.LastUpdated)
	}
	if page.Draft {
		t.Fatalf("Draft = true, want false")
	}
	assertStringSlice(t, "Tags", page.Tags, []string{"private", "test"})
	assertStringSlice(t, "Aliases", page.Aliases, []string{"Foo", "F."})
	assertStringSlice(t, "Sources", page.Sources, []string{"[[s-foo]]"})
	if got := page.Frontmatter["title"]; got != "Foo" {
		t.Fatalf("Frontmatter[title] = %q, want Foo", got)
	}
	if len(page.FrontRawLines) == 0 {
		t.Fatalf("FrontRawLines is empty")
	}
	if page.Body != "Lead paragraph with [[bar]] and [[baz|Baz Display]].\n" {
		t.Fatalf("Body = %q", page.Body)
	}
	wantLinks := []Link{
		{Target: "bar", Display: "", Raw: "[[bar]]"},
		{Target: "baz", Display: "Baz Display", Raw: "[[baz|Baz Display]]"},
	}
	if !reflect.DeepEqual(page.Links, wantLinks) {
		t.Fatalf("Links = %#v, want %#v", page.Links, wantLinks)
	}
}

func TestDiscoverPagesSortsMarkdownFilesAndIgnoresNonMarkdown(t *testing.T) {
	contentDir := t.TempDir()
	writePage(t, contentDir, "topics/zeta.md", "---\ntitle: Zeta\n---\n")
	writePage(t, contentDir, "entities/alpha.md", "---\ntitle: Alpha\n---\n")
	writePage(t, contentDir, "notes.txt", "not markdown")
	writePage(t, contentDir, "entities/beta.markdown", "not included")

	pages, err := DiscoverPages(contentDir)
	if err != nil {
		t.Fatalf("DiscoverPages() error = %v", err)
	}

	var relPaths []string
	for _, page := range pages {
		relPaths = append(relPaths, page.RelPath)
	}
	want := []string{"entities/alpha.md", "topics/zeta.md"}
	if !reflect.DeepEqual(relPaths, want) {
		t.Fatalf("RelPaths = %#v, want %#v", relPaths, want)
	}
}

func TestParsePageBlockLists(t *testing.T) {
	contentDir := t.TempDir()
	path := writePage(t, contentDir, "entities/foo.md", `---
title: Foo
tags:
  - private
aliases:
  - Foo
sources:
  - "[[s-foo]]"
---
Body.
`)

	page, err := ParsePage(path, contentDir)
	if err != nil {
		t.Fatalf("ParsePage() error = %v", err)
	}

	assertStringSlice(t, "Tags", page.Tags, []string{"private"})
	assertStringSlice(t, "Aliases", page.Aliases, []string{"Foo"})
	assertStringSlice(t, "Sources", page.Sources, []string{"[[s-foo]]"})
}

func writePage(t *testing.T, root, relPath, content string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func assertStringSlice(t *testing.T, name string, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %#v, want %#v", name, got, want)
	}
}
