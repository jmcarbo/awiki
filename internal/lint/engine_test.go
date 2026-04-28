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

func TestRunOnlySynthSuppressesCoreRules(t *testing.T) {
	contentDir := t.TempDir()
	writeLintPage(t, contentDir, "entities/source.md", pageContent("Source", "entity", nil, nil, longBody("Source references [[missing-target]] for coverage.")))

	collector, code := Run(Options{ContentDir: contentDir, Only: "synth"})

	if code != 0 {
		t.Fatalf("Run() code = %d, want 0; diagnostics = %#v", code, collector.Diagnostics)
	}
	assertNoDiagnosticContains(t, collector, Error, "entities/source.md", "broken wikilink")
	if len(collector.Diagnostics) != 0 {
		t.Fatalf("Diagnostics = %#v, want none for non-core only namespace", collector.Diagnostics)
	}
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

func TestRunSourcesFrontmatterLinksCountAsInboundForOrphans(t *testing.T) {
	contentDir := t.TempDir()
	writeLintPage(t, contentDir, "sources/source.md", pageContent("Source", "source", nil, nil, longBody("Source is referenced only from another page frontmatter.")))
	writeLintPage(t, contentDir, "entities/referrer.md", pageContentWithSources("Referrer", "entity", nil, nil, []string{"[[source]]"}, longBody("Referrer has no body wikilink to source.")))

	collector, code := Run(Options{ContentDir: contentDir})

	if code != 0 {
		t.Fatalf("Run() code = %d, want 0; diagnostics = %#v", code, collector.Diagnostics)
	}
	assertNoDiagnosticContains(t, collector, Info, "sources/source.md", "orphan")
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

func TestRunFixAddsLastUpdatedAfterDate(t *testing.T) {
	contentDir := t.TempDir()
	writeLintPage(t, contentDir, "entities/source.md", pageContentWithoutField("Source", "entity", "last_updated", longBody("Source has enough body text for lint.")))

	collector, code := Run(Options{ContentDir: contentDir, Fix: true, Today: "2026-04-28"})

	if code != 0 {
		t.Fatalf("Run() code = %d, want 0; diagnostics = %#v", code, collector.Diagnostics)
	}
	content := readLintPage(t, contentDir, "entities/source.md")
	want := "date: 2026-04-28\nlast_updated: 2026-04-28\ntype: entity"
	if !strings.Contains(content, want) {
		t.Fatalf("fixed content missing inserted last_updated after date:\n%s", content)
	}
	assertFixContains(t, collector, "entities/source.md", "added last_updated: 2026-04-28")
}

func TestRunFixLeavesPageWithoutDateUnchanged(t *testing.T) {
	contentDir := t.TempDir()
	writeLintPage(t, contentDir, "entities/source.md", pageContentWithoutField("Source", "entity", "date", longBody("Source has enough body text for lint.")))
	before := readLintPage(t, contentDir, "entities/source.md")

	collector, code := Run(Options{ContentDir: contentDir, Fix: true, Today: "2026-04-28"})

	if code != 0 {
		t.Fatalf("Run() code = %d, want 0; diagnostics = %#v", code, collector.Diagnostics)
	}
	after := readLintPage(t, contentDir, "entities/source.md")
	if after != before {
		t.Fatalf("content changed without date:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if len(collector.Fixes) != 0 {
		t.Fatalf("Fixes = %#v, want none", collector.Fixes)
	}
}

func TestRunFixDoesNotUseBodyDateWhenFrontmatterDateIsNotExact(t *testing.T) {
	contentDir := t.TempDir()
	writeLintPage(t, contentDir, "entities/source.md", strings.Join([]string{
		"---",
		"title: \"Source\"",
		"  date: 2026-04-28",
		"type: entity",
		"tags: []",
		"aliases: []",
		"sources: []",
		"draft: false",
		"---",
		"date: body-value",
		longBody("Source has enough body text for lint."),
		"",
	}, "\n"))
	before := readLintPage(t, contentDir, "entities/source.md")

	collector, code := Run(Options{ContentDir: contentDir, Fix: true, Today: "2026-04-28"})

	if code != 0 {
		t.Fatalf("Run() code = %d, want 0; diagnostics = %#v", code, collector.Diagnostics)
	}
	after := readLintPage(t, contentDir, "entities/source.md")
	if after != before {
		t.Fatalf("content changed using body date line:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if len(collector.Fixes) != 0 {
		t.Fatalf("Fixes = %#v, want none", collector.Fixes)
	}
}

func TestRunFixLeavesExistingLastUpdatedUnchanged(t *testing.T) {
	contentDir := t.TempDir()
	writeLintPage(t, contentDir, "entities/source.md", pageContent("Source", "entity", nil, nil, longBody("Source has enough body text for lint.")))
	before := readLintPage(t, contentDir, "entities/source.md")

	collector, code := Run(Options{ContentDir: contentDir, Fix: true, Today: "2026-04-28"})

	if code != 0 {
		t.Fatalf("Run() code = %d, want 0; diagnostics = %#v", code, collector.Diagnostics)
	}
	after := readLintPage(t, contentDir, "entities/source.md")
	if after != before {
		t.Fatalf("content changed with existing last_updated:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if len(collector.Fixes) != 0 {
		t.Fatalf("Fixes = %#v, want none", collector.Fixes)
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

func readLintPage(t *testing.T, root, relPath string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relPath))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	return string(data)
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

func pageContentWithoutField(title, typ, omittedField, body string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("title: \"" + title + "\"\n")
	if omittedField != "date" {
		b.WriteString("date: 2026-04-28\n")
	}
	if omittedField != "last_updated" {
		b.WriteString("last_updated: 2026-04-28\n")
	}
	b.WriteString("type: " + typ + "\n")
	b.WriteString("tags: []\n")
	b.WriteString("aliases: []\n")
	b.WriteString("sources: []\n")
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

func assertFixContains(t *testing.T, collector Collector, relPath, messagePart string) {
	t.Helper()
	for _, f := range collector.Fixes {
		if strings.HasSuffix(filepath.ToSlash(f.File), relPath) && strings.Contains(f.Message, messagePart) {
			return
		}
	}
	t.Fatalf("missing fix for %q containing %q; got %#v", relPath, messagePart, collector.Fixes)
}
