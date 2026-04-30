package ops

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupBuildRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "content", "entities"))
	mustWrite(t, filepath.Join(dir, "content", "entities", "foo.md"), `---
title: "Foo"
date: 2026-04-27
last_updated: 2026-04-27
type: entity
tags: []
aliases: [Effoh]
sources: []
draft: false
---

Sees [[bar]].
`)
	mustWrite(t, filepath.Join(dir, "content", "entities", "bar.md"), `---
title: "Bar"
date: 2026-04-27
last_updated: 2026-04-27
type: entity
tags: []
aliases: []
sources: []
draft: false
---

Sees [[foo]] and [[Effoh|alias-display]].
`)
	t.Setenv("AWIKI_REPO_ROOT", dir)
	return dir
}

func TestBuildEmitsSlugMap(t *testing.T) {
	repo := setupBuildRepo(t)
	rc := Build(BuildOptions{RepoRoot: repo, MapsOnly: true}, &bytes.Buffer{}, &bytes.Buffer{})
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	got := mustRead(t, filepath.Join(repo, ".awiki", "maps", "slug-to-path.tsv"))
	if !strings.Contains(got, "foo\tentities/foo.md") {
		t.Fatalf("missing foo entry: %q", got)
	}
	if !strings.Contains(got, "bar\tentities/bar.md") {
		t.Fatalf("missing bar entry: %q", got)
	}
}

func TestBuildRewritesWikilinks(t *testing.T) {
	repo := setupBuildRepo(t)
	rc := Build(BuildOptions{RepoRoot: repo}, &bytes.Buffer{}, &bytes.Buffer{})
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	got := mustRead(t, filepath.Join(repo, ".awiki", "build-content", "entities", "foo.md"))
	if !strings.Contains(got, "[Bar](/entities/bar/)") {
		t.Fatalf("unrewritten link: %s", got)
	}
}

func TestBuildResolvesAliasWikilink(t *testing.T) {
	repo := setupBuildRepo(t)
	rc := Build(BuildOptions{RepoRoot: repo}, &bytes.Buffer{}, &bytes.Buffer{})
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	got := mustRead(t, filepath.Join(repo, ".awiki", "build-content", "entities", "bar.md"))
	if !strings.Contains(got, "[alias-display](/entities/foo/)") {
		t.Fatalf("alias not resolved: %s", got)
	}
}

func TestBuildAliasListWithQuotedComma(t *testing.T) {
	repo := setupBuildRepo(t)
	mustWrite(t, filepath.Join(repo, "content", "entities", "qux.md"), `---
title: "Qux"
aliases: ["A", "B, with comma"]
draft: false
---

body
`)
	rc := Build(BuildOptions{RepoRoot: repo, MapsOnly: true}, &bytes.Buffer{}, &bytes.Buffer{})
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	got := mustRead(t, filepath.Join(repo, ".awiki", "maps", "alias-to-slug.tsv"))
	if !strings.Contains(got, "A\tqux") {
		t.Fatalf("missing A: %s", got)
	}
	if !strings.Contains(got, "B, with comma\tqux") {
		t.Fatalf("comma alias mis-split: %s", got)
	}
	count := strings.Count(got, "\tqux\n")
	if count != 2 {
		t.Fatalf("want 2 qux aliases; got %d (%s)", count, got)
	}
}

func TestBuildSingleQuotedTitleStripped(t *testing.T) {
	repo := setupBuildRepo(t)
	mustWrite(t, filepath.Join(repo, "content", "entities", "sq.md"), `---
title: 'Single Quoted'
aliases: []
draft: false
---

body
`)
	rc := Build(BuildOptions{RepoRoot: repo, MapsOnly: true}, &bytes.Buffer{}, &bytes.Buffer{})
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	got := mustRead(t, filepath.Join(repo, ".awiki", "maps", "slug-to-title.tsv"))
	if !strings.Contains(got, "sq\tSingle Quoted\n") {
		t.Fatalf("title not stripped: %q", got)
	}
	if strings.Contains(got, "'Single Quoted'") {
		t.Fatalf("quotes leaked: %q", got)
	}
}

func TestBuildPreservesCodeBlocks(t *testing.T) {
	repo := setupBuildRepo(t)
	mustWrite(t, filepath.Join(repo, "content", "entities", "code.md"), "---\n"+
		"title: \"Code\"\n"+
		"aliases: []\n"+
		"draft: false\n"+
		"---\n\n"+
		"Outside [[bar]] gets rewritten.\n\n"+
		"```\n"+
		"Inside fenced [[bar]] stays literal.\n"+
		"```\n\n"+
		"Inline `[[bar]]` also stays literal.\n")
	rc := Build(BuildOptions{RepoRoot: repo}, &bytes.Buffer{}, &bytes.Buffer{})
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	got := mustRead(t, filepath.Join(repo, ".awiki", "build-content", "entities", "code.md"))
	if !strings.Contains(got, "Outside [Bar](/entities/bar/) gets rewritten.") {
		t.Fatalf("outside not rewritten: %s", got)
	}
	if !strings.Contains(got, "Inside fenced [[bar]] stays literal.") {
		t.Fatalf("fenced rewritten: %s", got)
	}
	if !strings.Contains(got, "Inline `[[bar]]` also stays literal.") {
		t.Fatalf("inline code rewritten: %s", got)
	}
}

func TestBuildSkipsIndexFromSlugMap(t *testing.T) {
	repo := setupBuildRepo(t)
	mustMkdir(t, filepath.Join(repo, "content", "section"))
	mustWrite(t, filepath.Join(repo, "content", "section", "_index.md"), `---
title: "Section"
draft: false
---

intro
`)
	rc := Build(BuildOptions{RepoRoot: repo, MapsOnly: true}, &bytes.Buffer{}, &bytes.Buffer{})
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	got := mustRead(t, filepath.Join(repo, ".awiki", "maps", "slug-to-path.tsv"))
	if strings.Contains(got, "_index") {
		t.Fatalf("_index leaked into slug map: %s", got)
	}
	titles := mustRead(t, filepath.Join(repo, ".awiki", "maps", "slug-to-title.tsv"))
	if strings.Contains(titles, "_index\t") {
		t.Fatalf("_index leaked into titles map: %s", titles)
	}
}

func TestBuildMapsOnlyFlag(t *testing.T) {
	repo := setupBuildRepo(t)
	var stdout bytes.Buffer
	rc := Build(BuildOptions{RepoRoot: repo, MapsOnly: true}, &stdout, &bytes.Buffer{})
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	if !strings.Contains(stdout.String(), "BUILD-OK|maps-only=1") {
		t.Fatalf("missing BUILD-OK: %s", stdout.String())
	}
	// build-content/ should not exist.
	if _, err := os.Stat(filepath.Join(repo, ".awiki", "build-content", "entities")); err == nil {
		t.Fatalf("maps-only should skip build-content")
	}
}
