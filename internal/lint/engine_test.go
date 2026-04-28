package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunBrokenWikilinkEmitsErrorAndExitTwo(t *testing.T) {
	contentDir := t.TempDir()
	writeLintPage(t, contentDir, "entities/source.md", pageContent("Source", "entity", nil, nil, longBody("Source references [[missing-target]] for coverage.")))

	collector, code := Run(Options{ContentDir: contentDir})

	if code != 2 {
		t.Fatalf("Run() code = %d, want 2; diagnostics = %#v", code, collector.Diagnostics)
	}
	assertDiagnosticContains(t, collector, Error, "entities/source.md", "broken wikilink: [[missing-target]]")
}

func TestRunBrokenWikilinkIncludesFrontmatterSources(t *testing.T) {
	contentDir := t.TempDir()
	writeLintPage(t, contentDir, "entities/source.md", pageContentWithSources("Source", "entity", nil, nil, []string{"[[missing-source]]"}, longBody("Source has enough body text without body wikilinks.")))

	collector, code := Run(Options{ContentDir: contentDir})

	if code != 2 {
		t.Fatalf("Run() code = %d, want 2; diagnostics = %#v", code, collector.Diagnostics)
	}
	assertDiagnosticContains(t, collector, Error, "entities/source.md", "broken wikilink: [[missing-source]]")
	assertDiagnosticCount(t, collector, Error, "entities/source.md", "broken wikilink: [[missing-source]]", 1)
}

func TestRunDuplicateSlugEmitsErrorAndIgnoresDuplicateIndexSlug(t *testing.T) {
	contentDir := t.TempDir()
	writeLintPage(t, contentDir, "entities/dup.md", pageContent("Dup A", "entity", nil, nil, longBody("Duplicate slug source A.")))
	writeLintPage(t, contentDir, "topics/dup.md", pageContent("Dup B", "topic", nil, nil, longBody("Duplicate slug source B.")))
	writeLintPage(t, contentDir, "entities/_index.md", pageContent("Entities", "section-index", nil, nil, "Index."))
	writeLintPage(t, contentDir, "topics/_index.md", pageContent("Topics", "section-index", nil, nil, "Index."))

	collector, code := Run(Options{ContentDir: contentDir})

	if code != 2 {
		t.Fatalf("Run() code = %d, want 2; diagnostics = %#v", code, collector.Diagnostics)
	}
	assertDiagnosticContains(t, collector, Error, "topics/dup.md", "duplicate slug: dup")
	assertDiagnosticContains(t, collector, Error, "topics/dup.md", "also at")
	assertNoDiagnosticContains(t, collector, Error, "_index", "duplicate slug")
}

func TestRunPrivateTagOutsidePrivatePathEmitsWarn(t *testing.T) {
	contentDir := t.TempDir()
	writeLintPage(t, contentDir, "entities/secret.md", pageContent("Secret", "entity", []string{"private"}, nil, longBody("Private-tagged content outside a private directory.")))

	collector, code := Run(Options{ContentDir: contentDir})

	if code != 1 {
		t.Fatalf("Run() code = %d, want 1; diagnostics = %#v", code, collector.Diagnostics)
	}
	assertDiagnosticContains(t, collector, Warn, "entities/secret.md", "private tag outside private path")
}

func TestRunPageMissingFromCatalogEmitsWarnWhenCatalogExists(t *testing.T) {
	contentDir := t.TempDir()
	writeLintPage(t, contentDir, "catalog.md", pageContent("Catalog", "catalog", nil, nil, longBody("Catalog intentionally omits the page under test.")))
	writeLintPage(t, contentDir, "entities/alpha.md", pageContent("Alpha", "entity", nil, nil, longBody("Alpha has enough body text and should be listed.")))

	collector, code := Run(Options{ContentDir: contentDir})

	if code != 1 {
		t.Fatalf("Run() code = %d, want 1; diagnostics = %#v", code, collector.Diagnostics)
	}
	assertDiagnosticContains(t, collector, Warn, "entities/alpha.md", "missing from catalog")
}

func TestRunEmptyPageEmitsWarnForShortBody(t *testing.T) {
	contentDir := t.TempDir()
	writeLintPage(t, contentDir, "entities/empty.md", pageContent("Empty", "entity", nil, nil, "Too short."))

	collector, code := Run(Options{ContentDir: contentDir})

	if code != 1 {
		t.Fatalf("Run() code = %d, want 1; diagnostics = %#v", code, collector.Diagnostics)
	}
	assertDiagnosticContains(t, collector, Warn, "entities/empty.md", "empty page")
}

func TestRunWhitespaceOnlyBodyOverFiftyCharsDoesNotEmitEmptyPageWarn(t *testing.T) {
	contentDir := t.TempDir()
	writeLintPage(t, contentDir, "entities/spaces.md", pageContent("Spaces", "entity", nil, nil, strings.Repeat(" ", 60)))

	collector, code := Run(Options{ContentDir: contentDir})

	if code != 0 {
		t.Fatalf("Run() code = %d, want 0; diagnostics = %#v", code, collector.Diagnostics)
	}
	assertNoDiagnosticContains(t, collector, Warn, "entities/spaces.md", "empty page")
}

func TestRunOrphanPageEmitsInfoAndSectionIndexIsExempt(t *testing.T) {
	contentDir := t.TempDir()
	writeLintPage(t, contentDir, "entities/orphan.md", pageContent("Orphan", "entity", nil, nil, longBody("This page has no inbound links.")))
	writeLintPage(t, contentDir, "entities/_index.md", pageContent("Entities", "section-index", nil, nil, longBody("Section index pages are exempt from orphan warnings.")))

	collector, code := Run(Options{ContentDir: contentDir})

	if code != 0 {
		t.Fatalf("Run() code = %d, want 0; diagnostics = %#v", code, collector.Diagnostics)
	}
	assertDiagnosticContains(t, collector, Info, "entities/orphan.md", "orphan")
	assertNoDiagnosticContains(t, collector, Info, "entities/_index.md", "orphan")
}

func TestRunPrivateCatalogDoesNotEnableCatalogCoverage(t *testing.T) {
	contentDir := t.TempDir()
	writeLintPage(t, contentDir, "private/catalog.md", pageContent("Private Catalog", "catalog", nil, nil, longBody("Private catalog should not activate root catalog coverage.")))
	writeLintPage(t, contentDir, "entities/alpha.md", pageContent("Alpha", "entity", nil, nil, longBody("Alpha has enough body text and no root catalog exists.")))

	collector, code := Run(Options{ContentDir: contentDir})

	if code != 0 {
		t.Fatalf("Run() code = %d, want 0; diagnostics = %#v", code, collector.Diagnostics)
	}
	assertNoDiagnosticContains(t, collector, Warn, "entities/alpha.md", "missing from catalog")
}

func TestRunCleanTwoPageLinkedWikiExitsZero(t *testing.T) {
	contentDir := t.TempDir()
	writeLintPage(t, contentDir, "entities/alpha.md", pageContent("Alpha", "entity", nil, nil, longBody("Alpha links to [[beta]] so beta is not orphaned.")))
	writeLintPage(t, contentDir, "entities/beta.md", pageContent("Beta", "entity", nil, nil, longBody("Beta links back to [[alpha]] so alpha is not orphaned.")))

	collector, code := Run(Options{ContentDir: contentDir})

	if code != 0 {
		t.Fatalf("Run() code = %d, want 0; diagnostics = %#v", code, collector.Diagnostics)
	}
	if len(collector.Diagnostics) != 0 {
		t.Fatalf("Diagnostics = %#v, want none", collector.Diagnostics)
	}
}

func writeLintPage(t *testing.T, root, relPath, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func pageContent(title, typ string, tags, aliases []string, body string) string {
	return pageContentWithSources(title, typ, tags, aliases, nil, body)
}

func pageContentWithSources(title, typ string, tags, aliases, sources []string, body string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("title: \"" + title + "\"\n")
	b.WriteString("date: 2026-04-28\n")
	b.WriteString("last_updated: 2026-04-28\n")
	b.WriteString("type: " + typ + "\n")
	b.WriteString("tags: [" + strings.Join(tags, ", ") + "]\n")
	b.WriteString("aliases: [" + strings.Join(aliases, ", ") + "]\n")
	b.WriteString("sources: [" + quotedList(sources) + "]\n")
	b.WriteString("draft: false\n")
	b.WriteString("---\n")
	b.WriteString(body)
	b.WriteString("\n")
	return b.String()
}

func longBody(seed string) string {
	return seed + " This body is intentionally longer than fifty trimmed characters."
}

func quotedList(values []string) string {
	var quoted []string
	for _, value := range values {
		quoted = append(quoted, `"`+value+`"`)
	}
	return strings.Join(quoted, ", ")
}

func assertDiagnosticContains(t *testing.T, collector Collector, level Level, relPath, messagePart string) {
	t.Helper()
	for _, d := range collector.Diagnostics {
		if d.Level != level {
			continue
		}
		if relPath != "" && !strings.HasSuffix(filepath.ToSlash(d.File), relPath) {
			continue
		}
		if strings.Contains(d.Message, messagePart) {
			return
		}
	}
	t.Fatalf("missing %s diagnostic for %q containing %q; got %#v", level, relPath, messagePart, collector.Diagnostics)
}

func assertDiagnosticCount(t *testing.T, collector Collector, level Level, relPath, messagePart string, want int) {
	t.Helper()
	got := 0
	for _, d := range collector.Diagnostics {
		if d.Level == level && strings.HasSuffix(filepath.ToSlash(d.File), relPath) && strings.Contains(d.Message, messagePart) {
			got++
		}
	}
	if got != want {
		t.Fatalf("diagnostic count for %s %q containing %q = %d, want %d; got %#v", level, relPath, messagePart, got, want, collector.Diagnostics)
	}
}

func assertNoDiagnosticContains(t *testing.T, collector Collector, level Level, filePart, messagePart string) {
	t.Helper()
	for _, d := range collector.Diagnostics {
		if d.Level == level && strings.Contains(filepath.ToSlash(d.File), filePart) && strings.Contains(d.Message, messagePart) {
			t.Fatalf("unexpected %s diagnostic containing file %q and message %q: %#v", level, filePart, messagePart, d)
		}
	}
}
