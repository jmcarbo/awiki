# Go awiki Lint Core Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `awiki lint` as a Go-owned core lint workflow that preserves existing `scripts/lint.sh` compatibility while delegating non-core lint namespaces.

**Architecture:** Add a root Go module with a small CLI entrypoint, focused wiki parsing/indexing packages, a lint rule engine, and adapter functions for Hugo plus legacy namespace lint. Keep the bash script as a compatibility shim once the Go path passes the existing core lint tests.

**Tech Stack:** Go standard library, existing Bats tests, existing shell scripts for delegated synth/data/chart/task lint, Hugo as an external adapter.

---

## File Structure

- Create `go.mod`: root Go module for the `awiki` executable.
- Create `cmd/awiki/main.go`: process entrypoint and exit-code handling.
- Create `internal/cli/cli.go`: command dispatch and `lint` flag parsing.
- Create `internal/wiki/page.go`: markdown page model, discovery, frontmatter parsing, and wikilink extraction.
- Create `internal/wiki/index.go`: slug, alias, title, and inbound-link indexes.
- Create `internal/wiki/page_test.go`: parser and discovery tests.
- Create `internal/wiki/index_test.go`: index behavior tests.
- Create `internal/lint/diagnostic.go`: diagnostic collection, record formatting, summaries, and exit status.
- Create `internal/lint/options.go`: lint option model.
- Create `internal/lint/engine.go`: core lint orchestration.
- Create `internal/lint/rules.go`: core wiki lint rules.
- Create `internal/lint/fix.go`: mechanical `last_updated:` fix.
- Create `internal/lint/engine_test.go`: core lint engine tests.
- Create `internal/adapters/external.go`: process runner for Hugo and legacy shell adapters.
- Create `internal/adapters/external_test.go`: adapter argument tests with a fake runner.
- Modify `scripts/lint.sh`: add a guarded Go shim path after the Go implementation is proven.
- Modify `justfile` only if the shim is insufficient; prefer leaving the existing recipes untouched in this slice.

## Task 1: Go Module and CLI Skeleton

**Files:**
- Create: `go.mod`
- Create: `cmd/awiki/main.go`
- Create: `internal/cli/cli.go`

- [ ] **Step 1: Write the failing CLI smoke test**

Create `internal/cli/cli_test.go`:

```go
package cli

import (
	"bytes"
	"testing"
)

func TestRunRequiresCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit for missing command")
	}
	if stderr.String() == "" {
		t.Fatalf("expected usage text on stderr")
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"nope"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit for unknown command")
	}
	if got := stderr.String(); got == "" || !bytes.Contains([]byte(got), []byte("unknown command")) {
		t.Fatalf("stderr = %q, want unknown command message", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```sh
go test ./internal/cli
```

Expected: FAIL because no Go module or `internal/cli` package exists.

- [ ] **Step 3: Add the module and CLI skeleton**

Create `go.mod`:

```go
module awiki

go 1.22
```

Create `cmd/awiki/main.go`:

```go
package main

import (
	"os"

	"awiki/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
```

Create `internal/cli/cli.go`:

```go
package cli

import (
	"fmt"
	"io"
)

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: awiki <command> [args]")
		return 1
	}
	switch args[0] {
	case "lint":
		fmt.Fprintln(stderr, "awiki lint is not implemented yet")
		return 1
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		return 1
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:

```sh
go test ./internal/cli
```

Expected: PASS.

- [ ] **Step 5: Commit**

```sh
git add go.mod cmd/awiki/main.go internal/cli/cli.go internal/cli/cli_test.go
git commit -m "feat: add awiki go cli skeleton"
```

## Task 2: Lint Options and Diagnostic Compatibility

**Files:**
- Modify: `internal/cli/cli.go`
- Create: `internal/lint/options.go`
- Create: `internal/lint/diagnostic.go`
- Create: `internal/lint/diagnostic_test.go`

- [ ] **Step 1: Write failing diagnostic tests**

Create `internal/lint/diagnostic_test.go`:

```go
package lint

import (
	"strings"
	"testing"
)

func TestDiagnosticRecordFormat(t *testing.T) {
	d := Diagnostic{Level: Error, File: "content/entities/foo.md", Message: "broken wikilink: [[bar]]"}
	if got, want := d.Record(), "LINT|ERROR|content/entities/foo.md|broken wikilink: [[bar]]"; got != want {
		t.Fatalf("Record() = %q, want %q", got, want)
	}
}

func TestSummaryAndExitCode(t *testing.T) {
	var c Collector
	c.Add(Diagnostic{Level: Error, File: "a.md", Message: "bad"})
	c.Add(Diagnostic{Level: Warn, File: "b.md", Message: "warn"})
	c.Add(Diagnostic{Level: Info, File: "c.md", Message: "info"})
	if got, want := c.Summary(), "LINT-SUMMARY|errors=1|warnings=1|info=1"; got != want {
		t.Fatalf("Summary() = %q, want %q", got, want)
	}
	if got := c.ExitCode(); got != 2 {
		t.Fatalf("ExitCode() = %d, want 2", got)
	}
}

func TestFixRecordFormat(t *testing.T) {
	r := FixRecord{File: "content/entities/foo.md", Message: "added last_updated: 2026-04-28"}
	if got := r.Record(); !strings.HasPrefix(got, "FIX|content/entities/foo.md|added last_updated:") {
		t.Fatalf("Record() = %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```sh
go test ./internal/lint
```

Expected: FAIL because `internal/lint` does not exist.

- [ ] **Step 3: Implement options and diagnostics**

Create `internal/lint/options.go`:

```go
package lint

type Options struct {
	Fix            bool
	Only           string
	OnlyFile       string
	HugoCheck      bool
	AliasBuildOnly bool
	ContentDir     string
	RepoRoot       string
	Today          string
}
```

Create `internal/lint/diagnostic.go`:

```go
package lint

import "fmt"

type Level string

const (
	Error Level = "ERROR"
	Warn  Level = "WARN"
	Info  Level = "INFO"
)

type Diagnostic struct {
	Level   Level
	File    string
	Message string
}

func (d Diagnostic) Record() string {
	return fmt.Sprintf("LINT|%s|%s|%s", d.Level, d.File, d.Message)
}

type FixRecord struct {
	File    string
	Message string
}

func (r FixRecord) Record() string {
	return fmt.Sprintf("FIX|%s|%s", r.File, r.Message)
}

type Collector struct {
	Diagnostics []Diagnostic
	Fixes       []FixRecord
}

func (c *Collector) Add(d Diagnostic) {
	c.Diagnostics = append(c.Diagnostics, d)
}

func (c *Collector) AddFix(f FixRecord) {
	c.Fixes = append(c.Fixes, f)
}

func (c Collector) Counts() (errors int, warnings int, infos int) {
	for _, d := range c.Diagnostics {
		switch d.Level {
		case Error:
			errors++
		case Warn:
			warnings++
		case Info:
			infos++
		}
	}
	return errors, warnings, infos
}

func (c Collector) Summary() string {
	errors, warnings, infos := c.Counts()
	return fmt.Sprintf("LINT-SUMMARY|errors=%d|warnings=%d|info=%d", errors, warnings, infos)
}

func (c Collector) ExitCode() int {
	errors, _, _ := c.Counts()
	if errors > 0 {
		return 2
	}
	return 0
}
```

- [ ] **Step 4: Parse `awiki lint` flags**

Modify `internal/cli/cli.go`:

```go
package cli

import (
	"flag"
	"fmt"
	"io"

	"awiki/internal/lint"
)

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: awiki <command> [args]")
		return 1
	}
	switch args[0] {
	case "lint":
		return runLint(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		return 1
	}
}

func runLint(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("lint", flag.ContinueOnError)
	fs.SetOutput(stderr)
	opts := lint.Options{ContentDir: "content", RepoRoot: "."}
	fs.BoolVar(&opts.Fix, "fix", false, "apply mechanical fixes")
	fs.StringVar(&opts.Only, "only", "", "run only a lint namespace")
	fs.StringVar(&opts.OnlyFile, "file", "", "run against one file")
	fs.BoolVar(&opts.HugoCheck, "hugo-check", false, "run Hugo render check")
	fs.BoolVar(&opts.AliasBuildOnly, "alias-build-only", false, "build maps only")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() > 0 {
		opts.ContentDir = fs.Arg(0)
	}
	collector, code := lint.Run(opts)
	for _, fix := range collector.Fixes {
		fmt.Fprintln(stdout, fix.Record())
	}
	for _, d := range collector.Diagnostics {
		fmt.Fprintln(stdout, d.Record())
	}
	fmt.Fprintln(stdout, collector.Summary())
	return code
}
```

- [ ] **Step 5: Add temporary lint runner stub**

Create `internal/lint/engine.go`:

```go
package lint

func Run(opts Options) (Collector, int) {
	var c Collector
	return c, c.ExitCode()
}
```

- [ ] **Step 6: Run tests**

Run:

```sh
go test ./...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```sh
git add internal/cli/cli.go internal/lint/options.go internal/lint/diagnostic.go internal/lint/diagnostic_test.go internal/lint/engine.go
git commit -m "feat: add lint cli options and diagnostics"
```

## Task 3: Wiki Page Parser and Wikilink Extraction

**Files:**
- Create: `internal/wiki/page.go`
- Create: `internal/wiki/page_test.go`

- [ ] **Step 1: Write failing parser tests**

Create `internal/wiki/page_test.go`:

```go
package wiki

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParsePageFrontmatterAndBody(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "content", "entities", "foo.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	input := `---
title: "Foo"
date: 2026-04-27
last_updated: 2026-04-27
type: entity
tags: [private, test]
aliases: [Foo, F.]
sources: ["[[s-foo]]"]
draft: false
---

Foo links to [[bar]] and [[baz|Baz Display]].
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	page, err := ParsePage(path, filepath.Join(dir, "content"))
	if err != nil {
		t.Fatal(err)
	}
	if page.Slug != "foo" || page.Type != "entity" || page.Title != "Foo" {
		t.Fatalf("page = %#v", page)
	}
	if !reflect.DeepEqual(page.Tags, []string{"private", "test"}) {
		t.Fatalf("Tags = %#v", page.Tags)
	}
	if !reflect.DeepEqual(page.Aliases, []string{"Foo", "F."}) {
		t.Fatalf("Aliases = %#v", page.Aliases)
	}
	if !reflect.DeepEqual(page.Links, []Link{{Target: "bar"}, {Target: "baz", Display: "Baz Display"}}) {
		t.Fatalf("Links = %#v", page.Links)
	}
}

func TestDiscoverPagesSortsMarkdownFiles(t *testing.T) {
	dir := t.TempDir()
	content := filepath.Join(dir, "content")
	for _, rel := range []string{"concepts/b.md", "entities/a.md", "entities/skip.txt"} {
		path := filepath.Join(content, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("---\ntype: concept\n---\n\nbody"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pages, err := DiscoverPages(content)
	if err != nil {
		t.Fatal(err)
	}
	got := []string{pages[0].RelPath, pages[1].RelPath}
	if !reflect.DeepEqual(got, []string{"concepts/b.md", "entities/a.md"}) {
		t.Fatalf("paths = %#v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```sh
go test ./internal/wiki
```

Expected: FAIL because `internal/wiki` does not exist.

- [ ] **Step 3: Implement page parsing**

Create `internal/wiki/page.go`:

```go
package wiki

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Link struct {
	Target  string
	Display string
}

type Page struct {
	Path          string
	RelPath       string
	Slug          string
	Title         string
	Type          string
	Date          string
	LastUpdated   string
	Tags          []string
	Aliases       []string
	Sources       []string
	Draft         string
	Frontmatter   map[string]string
	FrontRawLines []string
	Body          string
	Links         []Link
}

var wikilinkRE = regexp.MustCompile(`\[\[([a-zA-Z0-9_.-]+)(?:\|([^\]]+))?\]\]`)

func DiscoverPages(contentDir string) ([]Page, error) {
	var paths []string
	if err := filepath.WalkDir(contentDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		paths = append(paths, path)
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Strings(paths)
	pages := make([]Page, 0, len(paths))
	for _, path := range paths {
		page, err := ParsePage(path, contentDir)
		if err != nil {
			return nil, err
		}
		pages = append(pages, page)
	}
	return pages, nil
}

func ParsePage(path string, contentDir string) (Page, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Page{}, err
	}
	rel, err := filepath.Rel(contentDir, path)
	if err != nil {
		rel = path
	}
	rel = filepath.ToSlash(rel)
	page := Page{
		Path:        path,
		RelPath:     rel,
		Slug:        strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		Frontmatter: map[string]string{},
	}
	front, body := splitFrontmatter(data)
	page.FrontRawLines = front
	page.Body = body
	for _, line := range front {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		page.Frontmatter[key] = value
		switch key {
		case "title":
			page.Title = trimYAMLScalar(value)
		case "type":
			page.Type = trimYAMLScalar(value)
		case "date":
			page.Date = trimYAMLScalar(value)
		case "last_updated":
			page.LastUpdated = trimYAMLScalar(value)
		case "draft":
			page.Draft = trimYAMLScalar(value)
		case "tags":
			page.Tags = parseInlineList(value)
		case "aliases":
			page.Aliases = parseInlineList(value)
		case "sources":
			page.Sources = parseInlineList(value)
		}
	}
	page.Links = ExtractLinks(body)
	return page, nil
}

func splitFrontmatter(data []byte) ([]string, string) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, string(data)
	}
	var end = -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return nil, string(data)
	}
	body := strings.Join(lines[end+1:], "\n")
	if len(lines) > end+1 {
		body += "\n"
	}
	return append([]string(nil), lines[1:end]...), body
}

func ExtractLinks(body string) []Link {
	matches := wikilinkRE.FindAllStringSubmatch(body, -1)
	links := make([]Link, 0, len(matches))
	for _, m := range matches {
		links = append(links, Link{Target: m[1], Display: m[2]})
	}
	return links
}

func parseInlineList(value string) []string {
	value = strings.TrimSpace(value)
	if value == "[]" || value == "" {
		return nil
	}
	value = strings.TrimPrefix(value, "[")
	value = strings.TrimSuffix(value, "]")
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = trimYAMLScalar(strings.TrimSpace(part))
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func trimYAMLScalar(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"`)
	value = strings.Trim(value, `'`)
	return value
}
```

- [ ] **Step 4: Run parser tests**

Run:

```sh
go test ./internal/wiki
```

Expected: PASS.

- [ ] **Step 5: Commit**

```sh
git add internal/wiki/page.go internal/wiki/page_test.go
git commit -m "feat: parse wiki markdown pages in go"
```

## Task 4: Wiki Indexes for Slugs, Aliases, Links, and Catalog

**Files:**
- Create: `internal/wiki/index.go`
- Create: `internal/wiki/index_test.go`

- [ ] **Step 1: Write failing index tests**

Create `internal/wiki/index_test.go`:

```go
package wiki

import "testing"

func TestBuildIndexResolvesSlugsAndAliases(t *testing.T) {
	pages := []Page{
		{RelPath: "entities/foo.md", Slug: "foo", Type: "entity", Aliases: []string{"Foo"}, Links: []Link{{Target: "bar"}}},
		{RelPath: "entities/bar.md", Slug: "bar", Type: "entity"},
	}
	idx := BuildIndex(pages)
	if got, ok := idx.Resolve("foo"); !ok || got.Slug != "foo" {
		t.Fatalf("Resolve(foo) = %#v, %v", got, ok)
	}
	if got, ok := idx.Resolve("Foo"); !ok || got.Slug != "foo" {
		t.Fatalf("Resolve(Foo) = %#v, %v", got, ok)
	}
	if got := idx.Inbound["bar"]; len(got) != 1 || got[0].Slug != "foo" {
		t.Fatalf("Inbound[bar] = %#v", got)
	}
}

func TestDuplicateSlugIgnoresIndexPages(t *testing.T) {
	pages := []Page{
		{RelPath: "entities/_index.md", Slug: "_index", Type: "section-index"},
		{RelPath: "concepts/_index.md", Slug: "_index", Type: "section-index"},
		{RelPath: "entities/foo.md", Slug: "foo", Type: "entity"},
		{RelPath: "concepts/foo.md", Slug: "foo", Type: "concept"},
	}
	idx := BuildIndex(pages)
	if len(idx.DuplicateSlugs["_index"]) != 0 {
		t.Fatalf("_index duplicates should be ignored: %#v", idx.DuplicateSlugs["_index"])
	}
	if len(idx.DuplicateSlugs["foo"]) != 2 {
		t.Fatalf("foo duplicate count = %d", len(idx.DuplicateSlugs["foo"]))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```sh
go test ./internal/wiki
```

Expected: FAIL because index types do not exist.

- [ ] **Step 3: Implement indexes**

Create `internal/wiki/index.go`:

```go
package wiki

type Index struct {
	Pages          []Page
	BySlug         map[string]Page
	Aliases        map[string]string
	AliasCollision map[string][]Page
	DuplicateSlugs map[string][]Page
	Inbound        map[string][]Page
	CatalogBody    string
}

func BuildIndex(pages []Page) Index {
	idx := Index{
		Pages:          pages,
		BySlug:         map[string]Page{},
		Aliases:        map[string]string{},
		AliasCollision: map[string][]Page{},
		DuplicateSlugs: map[string][]Page{},
		Inbound:        map[string][]Page{},
	}
	slugSeen := map[string][]Page{}
	aliasSeen := map[string][]Page{}
	for _, page := range pages {
		if page.Slug == "catalog" {
			idx.CatalogBody = page.Body
		}
		slugSeen[page.Slug] = append(slugSeen[page.Slug], page)
		if _, exists := idx.BySlug[page.Slug]; !exists {
			idx.BySlug[page.Slug] = page
		}
		for _, alias := range page.Aliases {
			aliasSeen[alias] = append(aliasSeen[alias], page)
		}
	}
	for slug, matches := range slugSeen {
		if slug == "_index" {
			continue
		}
		if len(matches) > 1 {
			idx.DuplicateSlugs[slug] = matches
		}
	}
	for alias, matches := range aliasSeen {
		if len(matches) == 1 {
			idx.Aliases[alias] = matches[0].Slug
			continue
		}
		idx.AliasCollision[alias] = matches
	}
	for _, page := range pages {
		for _, link := range page.Links {
			if target, ok := idx.Resolve(link.Target); ok {
				idx.Inbound[target.Slug] = append(idx.Inbound[target.Slug], page)
			}
		}
	}
	return idx
}

func (idx Index) Resolve(target string) (Page, bool) {
	if page, ok := idx.BySlug[target]; ok {
		return page, true
	}
	if slug, ok := idx.Aliases[target]; ok {
		page, exists := idx.BySlug[slug]
		return page, exists
	}
	return Page{}, false
}
```

- [ ] **Step 4: Run index tests**

Run:

```sh
go test ./internal/wiki
```

Expected: PASS.

- [ ] **Step 5: Commit**

```sh
git add internal/wiki/index.go internal/wiki/index_test.go
git commit -m "feat: build wiki link indexes in go"
```

## Task 5: Core Lint Rules

**Files:**
- Modify: `internal/lint/engine.go`
- Create: `internal/lint/rules.go`
- Create: `internal/lint/engine_test.go`

- [ ] **Step 1: Write failing core rule tests**

Create `internal/lint/engine_test.go`:

```go
package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePage(t *testing.T, contentDir string, rel string, text string) {
	t.Helper()
	path := filepath.Join(contentDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunDetectsBrokenLinkAndDuplicateSlug(t *testing.T) {
	root := t.TempDir()
	content := filepath.Join(root, "content")
	writePage(t, content, "entities/foo.md", "---\ntitle: Foo\ntype: entity\ntags: []\naliases: []\nsources: []\ndraft: false\n---\n\n[[missing]]\n")
	writePage(t, content, "concepts/foo.md", "---\ntitle: Foo 2\ntype: concept\ntags: []\naliases: []\nsources: []\ndraft: false\n---\n\nBody\n")
	c, code := Run(Options{ContentDir: content, RepoRoot: root})
	if code != 2 {
		t.Fatalf("code = %d, want 2; diagnostics=%#v", code, c.Diagnostics)
	}
	if !hasMessage(c, "broken wikilink: [[missing]]") {
		t.Fatalf("missing broken-link diagnostic: %#v", c.Diagnostics)
	}
	if !hasMessage(c, "duplicate slug: foo") {
		t.Fatalf("missing duplicate-slug diagnostic: %#v", c.Diagnostics)
	}
}

func TestRunWarnsOnPrivacyAndCatalog(t *testing.T) {
	root := t.TempDir()
	content := filepath.Join(root, "content")
	writePage(t, content, "catalog.md", "---\ntitle: Catalog\ntype: catalog\n---\n\n# Catalog\n")
	writePage(t, content, "entities/leaky.md", "---\ntitle: Leaky\ndate: 2026-04-27\nlast_updated: 2026-04-27\ntype: entity\ntags: [private]\naliases: []\nsources: []\ndraft: false\n---\n\nLeaky [[leaky]] self-reference.\n")
	c, _ := Run(Options{ContentDir: content, RepoRoot: root})
	if !hasMessage(c, "private tag outside private path") {
		t.Fatalf("missing privacy warning: %#v", c.Diagnostics)
	}
	if !hasMessage(c, "missing from catalog") {
		t.Fatalf("missing catalog warning: %#v", c.Diagnostics)
	}
}

func hasMessage(c Collector, substr string) bool {
	for _, d := range c.Diagnostics {
		if strings.Contains(d.Message, substr) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```sh
go test ./internal/lint
```

Expected: FAIL because rules are not implemented.

- [ ] **Step 3: Implement core rules**

Create `internal/lint/rules.go`:

```go
package lint

import (
	"fmt"
	"strings"

	"awiki/internal/wiki"
)

func runCoreRules(c *Collector, pages []wiki.Page, idx wiki.Index) {
	for slug, matches := range idx.DuplicateSlugs {
		if len(matches) < 2 {
			continue
		}
		first := matches[0]
		for _, page := range matches[1:] {
			c.Add(Diagnostic{Level: Error, File: page.RelPath, Message: fmt.Sprintf("duplicate slug: %s also at %s", slug, first.RelPath)})
		}
	}
	for alias, matches := range idx.AliasCollision {
		for _, page := range matches {
			c.Add(Diagnostic{Level: Error, File: page.RelPath, Message: fmt.Sprintf("duplicate alias: %s", alias)})
		}
	}
	for _, page := range pages {
		runPageRules(c, page, idx)
	}
}

func runPageRules(c *Collector, page wiki.Page, idx wiki.Index) {
	if strings.TrimSpace(page.Body) == "" {
		c.Add(Diagnostic{Level: Warn, File: page.RelPath, Message: "empty page"})
	}
	if hasPrivateTag(page) && !isPrivatePath(page.RelPath) {
		c.Add(Diagnostic{Level: Warn, File: page.RelPath, Message: "private tag outside private path"})
	}
	for _, link := range page.Links {
		if _, ok := idx.Resolve(link.Target); !ok {
			c.Add(Diagnostic{Level: Error, File: page.RelPath, Message: fmt.Sprintf("broken wikilink: [[%s]]", link.Target)})
		}
	}
	if shouldCheckOrphan(page) && len(idx.Inbound[page.Slug]) == 0 {
		c.Add(Diagnostic{Level: Info, File: page.RelPath, Message: "orphan page (no inbound wikilinks)"})
	}
	if shouldCheckCatalog(page, idx) && !strings.Contains(idx.CatalogBody, "[["+page.Slug+"]]" ) {
		c.Add(Diagnostic{Level: Warn, File: page.RelPath, Message: "missing from catalog"})
	}
}

func hasPrivateTag(page wiki.Page) bool {
	for _, tag := range page.Tags {
		if tag == "private" {
			return true
		}
	}
	return false
}

func isPrivatePath(rel string) bool {
	return rel == "private.md" || strings.HasPrefix(rel, "private/")
}

func shouldCheckOrphan(page wiki.Page) bool {
	switch page.Type {
	case "log", "catalog", "section-index":
		return false
	}
	if page.Slug == "_index" || page.Slug == "catalog" {
		return false
	}
	return true
}

func shouldCheckCatalog(page wiki.Page, idx wiki.Index) bool {
	if idx.CatalogBody == "" {
		return false
	}
	switch page.Type {
	case "catalog", "log", "section-index":
		return false
	}
	if page.Slug == "_index" || page.Slug == "catalog" {
		return false
	}
	return true
}
```

Modify `internal/lint/engine.go`:

```go
package lint

import "awiki/internal/wiki"

func Run(opts Options) (Collector, int) {
	var c Collector
	pages, err := wiki.DiscoverPages(opts.ContentDir)
	if err != nil {
		c.Add(Diagnostic{Level: Error, File: opts.ContentDir, Message: err.Error()})
		return c, c.ExitCode()
	}
	idx := wiki.BuildIndex(pages)
	runCoreRules(&c, pages, idx)
	return c, c.ExitCode()
}
```

- [ ] **Step 4: Run lint Go tests**

Run:

```sh
go test ./internal/lint ./internal/wiki
```

Expected: PASS.

- [ ] **Step 5: Commit**

```sh
git add internal/lint/engine.go internal/lint/rules.go internal/lint/engine_test.go
git commit -m "feat: port core wiki lint rules to go"
```

## Task 6: Mechanical `--fix` for `last_updated`

**Files:**
- Modify: `internal/lint/engine.go`
- Create: `internal/lint/fix.go`
- Modify: `internal/lint/engine_test.go`

- [ ] **Step 1: Write failing fix tests**

Append to `internal/lint/engine_test.go`:

```go
func TestFixAddsLastUpdatedAfterDate(t *testing.T) {
	root := t.TempDir()
	content := filepath.Join(root, "content")
	writePage(t, content, "entities/needs-fix.md", "---\ntitle: Needs Fix\ndate: 2026-01-01\ntype: entity\ntags: []\naliases: []\nsources: []\ndraft: false\n---\n\nBody [[needs-fix]].\n")
	c, _ := Run(Options{ContentDir: content, RepoRoot: root, Fix: true, Today: "2026-04-28"})
	path := filepath.Join(content, "entities", "needs-fix.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); !strings.Contains(got, "date: 2026-01-01\nlast_updated: 2026-04-28\ntype: entity") {
		t.Fatalf("file after fix:\n%s", got)
	}
	if len(c.Fixes) != 1 || !strings.Contains(c.Fixes[0].Message, "added last_updated: 2026-04-28") {
		t.Fatalf("fixes = %#v", c.Fixes)
	}
}

func TestFixNoopsWithoutDate(t *testing.T) {
	root := t.TempDir()
	content := filepath.Join(root, "content")
	writePage(t, content, "entities/no-date.md", "---\ntitle: No Date\ntype: entity\ntags: []\naliases: []\nsources: []\ndraft: false\n---\n\nBody [[no-date]].\n")
	path := filepath.Join(content, "entities", "no-date.md")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := Run(Options{ContentDir: content, RepoRoot: root, Fix: true, Today: "2026-04-28"})
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("file changed without date anchor")
	}
	if len(c.Fixes) != 0 {
		t.Fatalf("fixes = %#v", c.Fixes)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```sh
go test ./internal/lint
```

Expected: FAIL because `--fix` is not implemented.

- [ ] **Step 3: Implement fix application**

Create `internal/lint/fix.go`:

```go
package lint

import (
	"os"
	"strings"
	"time"

	"awiki/internal/wiki"
)

func applyFixes(c *Collector, pages []wiki.Page, opts Options) {
	today := opts.Today
	if today == "" {
		today = time.Now().Format("2006-01-02")
	}
	for _, page := range pages {
		if page.LastUpdated != "" {
			continue
		}
		if page.Date == "" {
			continue
		}
		data, err := os.ReadFile(page.Path)
		if err != nil {
			c.Add(Diagnostic{Level: Error, File: page.RelPath, Message: err.Error()})
			continue
		}
		lines := strings.SplitAfter(string(data), "\n")
		var out strings.Builder
		changed := false
		for _, line := range lines {
			out.WriteString(line)
			if !changed && strings.HasPrefix(line, "date: ") {
				out.WriteString("last_updated: " + today + "\n")
				changed = true
			}
		}
		if !changed {
			continue
		}
		if err := os.WriteFile(page.Path, []byte(out.String()), 0o644); err != nil {
			c.Add(Diagnostic{Level: Error, File: page.RelPath, Message: err.Error()})
			continue
		}
		c.AddFix(FixRecord{File: page.RelPath, Message: "added last_updated: " + today})
	}
}
```

Modify `internal/lint/engine.go`:

```go
package lint

import "awiki/internal/wiki"

func Run(opts Options) (Collector, int) {
	var c Collector
	pages, err := wiki.DiscoverPages(opts.ContentDir)
	if err != nil {
		c.Add(Diagnostic{Level: Error, File: opts.ContentDir, Message: err.Error()})
		return c, c.ExitCode()
	}
	if opts.Fix {
		applyFixes(&c, pages, opts)
		pages, err = wiki.DiscoverPages(opts.ContentDir)
		if err != nil {
			c.Add(Diagnostic{Level: Error, File: opts.ContentDir, Message: err.Error()})
			return c, c.ExitCode()
		}
	}
	idx := wiki.BuildIndex(pages)
	runCoreRules(&c, pages, idx)
	return c, c.ExitCode()
}
```

- [ ] **Step 4: Run tests**

Run:

```sh
go test ./internal/lint ./internal/wiki
```

Expected: PASS.

- [ ] **Step 5: Commit**

```sh
git add internal/lint/engine.go internal/lint/fix.go internal/lint/engine_test.go
git commit -m "feat: add go lint last_updated fix"
```

## Task 7: External Adapters for Deferred Lint and Hugo

**Files:**
- Create: `internal/adapters/external.go`
- Create: `internal/adapters/external_test.go`
- Modify: `internal/lint/options.go`
- Modify: `internal/lint/engine.go`
- Modify: `internal/cli/cli.go`

- [ ] **Step 1: Write failing adapter tests**

Create `internal/adapters/external_test.go`:

```go
package adapters

import (
	"context"
	"reflect"
	"testing"
)

type fakeRunner struct {
	name string
	args []string
	out  string
	code int
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) (string, int, error) {
	f.name = name
	f.args = args
	return f.out, f.code, nil
}

func TestLegacyLintArgs(t *testing.T) {
	r := &fakeRunner{out: "LINT-SUMMARY|errors=0|warnings=0|info=0\n"}
	_, _, err := LegacyLint(context.Background(), r, ".", "synth", "content/synthesis/a.md", "content", false)
	if err != nil {
		t.Fatal(err)
	}
	if r.name != "env" {
		t.Fatalf("name = %q", r.name)
	}
	want := []string{"AWIKI_LINT_LEGACY=1", "bash", "scripts/lint.sh", "--only=synth", "--file=content/synthesis/a.md", "content"}
	if !reflect.DeepEqual(r.args, want) {
		t.Fatalf("args = %#v, want %#v", r.args, want)
	}
}

func TestHugoCheckRunsHugo(t *testing.T) {
	r := &fakeRunner{out: ""}
	_, _, err := HugoCheck(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if r.name != "hugo" {
		t.Fatalf("name = %q", r.name)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```sh
go test ./internal/adapters
```

Expected: FAIL because adapter package does not exist.

- [ ] **Step 3: Implement adapters**

Create `internal/adapters/external.go`:

```go
package adapters

import (
	"context"
	"os/exec"
)

type Runner interface {
	Run(ctx context.Context, name string, args ...string) (output string, code int, err error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) (string, int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0, nil
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return string(out), exit.ExitCode(), nil
	}
	return string(out), 1, err
}

func LegacyLint(ctx context.Context, r Runner, repoRoot string, only string, onlyFile string, contentDir string, fix bool) (string, int, error) {
	args := []string{"AWIKI_LINT_LEGACY=1", "bash", "scripts/lint.sh"}
	if only != "" {
		args = append(args, "--only="+only)
	}
	if fix {
		args = append(args, "--fix")
	}
	if onlyFile != "" {
		args = append(args, "--file="+onlyFile)
	}
	args = append(args, contentDir)
	return r.Run(ctx, "env", args...)
}

func HugoCheck(ctx context.Context, r Runner) (string, int, error) {
	return r.Run(ctx, "hugo", "--minify")
}
```

- [ ] **Step 4: Add adapter injection to lint options**

Modify `internal/lint/options.go`:

```go
package lint

import "awiki/internal/adapters"

type Options struct {
	Fix            bool
	Only           string
	OnlyFile       string
	HugoCheck      bool
	AliasBuildOnly bool
	ContentDir     string
	RepoRoot       string
	Today          string
	Runner         adapters.Runner
}
```

- [ ] **Step 5: Wire deferred namespace and Hugo behavior**

Modify `internal/lint/engine.go`:

```go
package lint

import (
	"context"
	"strings"

	"awiki/internal/adapters"
	"awiki/internal/wiki"
)

func Run(opts Options) (Collector, int) {
	var c Collector
	runner := opts.Runner
	if runner == nil {
		runner = adapters.ExecRunner{}
	}
	if isDeferredNamespace(opts.Only) || opts.AliasBuildOnly {
		out, code, err := adapters.LegacyLint(context.Background(), runner, opts.RepoRoot, opts.Only, opts.OnlyFile, opts.ContentDir, opts.Fix)
		importLegacyOutput(&c, out)
		if err != nil {
			c.Add(Diagnostic{Level: Error, File: "lint", Message: err.Error()})
		}
		if code != 0 && c.ExitCode() == 0 {
			return c, code
		}
		return c, c.ExitCode()
	}
	pages, err := wiki.DiscoverPages(opts.ContentDir)
	if err != nil {
		c.Add(Diagnostic{Level: Error, File: opts.ContentDir, Message: err.Error()})
		return c, c.ExitCode()
	}
	if opts.Fix {
		applyFixes(&c, pages, opts)
		pages, err = wiki.DiscoverPages(opts.ContentDir)
		if err != nil {
			c.Add(Diagnostic{Level: Error, File: opts.ContentDir, Message: err.Error()})
			return c, c.ExitCode()
		}
	}
	idx := wiki.BuildIndex(pages)
	runCoreRules(&c, pages, idx)
	if opts.HugoCheck {
		out, code, err := adapters.HugoCheck(context.Background(), runner)
		if err != nil {
			c.Add(Diagnostic{Level: Error, File: "hugo", Message: err.Error()})
		}
		if code != 0 {
			msg := strings.TrimSpace(out)
			if msg == "" {
				msg = "hugo render failed"
			}
			c.Add(Diagnostic{Level: Error, File: "hugo", Message: msg})
		}
	}
	return c, c.ExitCode()
}

func isDeferredNamespace(only string) bool {
	switch only {
	case "synth", "data", "chart", "task":
		return true
	default:
		return false
	}
}

func importLegacyOutput(c *Collector, out string) {
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "LINT|ERROR|") {
			parts := strings.SplitN(line, "|", 4)
			if len(parts) == 4 {
				c.Add(Diagnostic{Level: Error, File: parts[2], Message: parts[3]})
			}
		} else if strings.HasPrefix(line, "LINT|WARN|") {
			parts := strings.SplitN(line, "|", 4)
			if len(parts) == 4 {
				c.Add(Diagnostic{Level: Warn, File: parts[2], Message: parts[3]})
			}
		} else if strings.HasPrefix(line, "LINT|INFO|") {
			parts := strings.SplitN(line, "|", 4)
			if len(parts) == 4 {
				c.Add(Diagnostic{Level: Info, File: parts[2], Message: parts[3]})
			}
		}
	}
}
```

- [ ] **Step 6: Run tests**

Run:

```sh
go test ./...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```sh
git add internal/adapters/external.go internal/adapters/external_test.go internal/lint/options.go internal/lint/engine.go internal/cli/cli.go
git commit -m "feat: add lint external adapters"
```

## Task 8: Compatibility Against Existing Core Bats Tests

**Files:**
- Modify: `scripts/lint.sh`
- Modify: `internal/lint/rules.go` as needed for exact compatibility gaps.
- Modify: `internal/wiki/page.go` as needed for exact parsing gaps.

- [ ] **Step 1: Run current core lint Bats tests against shell baseline**

Run:

```sh
bats tests/lint_test.bats
```

Expected: PASS before changing the shim. If this fails, stop and fix the current repo baseline before wiring Go.

- [ ] **Step 2: Build the Go executable**

Run:

```sh
go build -o bin/awiki ./cmd/awiki
./bin/awiki lint tests/fixtures/wiki-broken/content; test "$?" -eq 2
./bin/awiki lint "$(mktemp -d)/missing-content"; test "$?" -eq 2
```

Expected: first command emits a `LINT-SUMMARY` and exits `2`; second command emits a lint error for the unreadable content directory and exits `2`.

- [ ] **Step 3: Add guarded Go shim to `scripts/lint.sh`**

Insert this block after argument parsing and before the current lint implementation in `scripts/lint.sh`:

```bash
if [[ "${AWIKI_LINT_LEGACY:-0}" != "1" && -x "./bin/awiki" ]]; then
  exec ./bin/awiki lint "$@"
fi
```

If the block needs original arguments after parsing mutates `$@`, place it before the `while [[ $# -gt 0 ]]` loop instead:

```bash
if [[ "${AWIKI_LINT_LEGACY:-0}" != "1" && -x "./bin/awiki" ]]; then
  exec ./bin/awiki lint "$@"
fi
```

Use the pre-parse placement if the post-parse placement changes flag behavior.

- [ ] **Step 4: Run core Bats tests through the Go shim**

Run:

```sh
go build -o bin/awiki ./cmd/awiki
bats tests/lint_test.bats
```

Expected: PASS. If a test fails, compare `AWIKI_LINT_LEGACY=1 bash scripts/lint.sh ...` with `./bin/awiki lint ...` and adjust Go output or ordering only enough to restore compatibility.

- [ ] **Step 5: Run delegated synth lint tests**

Run:

```sh
go build -o bin/awiki ./cmd/awiki
bats tests/lint_synth_test.bats
```

Expected: PASS. `lint.sh --only=synth` must still reach the legacy synth rules.

- [ ] **Step 6: Run Go tests**

Run:

```sh
go test ./...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```sh
git add scripts/lint.sh internal/lint/rules.go internal/wiki/page.go
git commit -m "feat: route lint workflow through awiki go core"
```

## Task 9: Final Verification and Handoff Notes

**Files:**
- Modify: `docs/superpowers/plans/2026-04-28-go-awiki-lint-core.md` only if execution reveals a required correction to the plan.

- [ ] **Step 1: Run final verification**

Run:

```sh
go test ./...
go build -o bin/awiki ./cmd/awiki
bats tests/lint_test.bats
bats tests/lint_synth_test.bats
```

Expected: all commands PASS.

- [ ] **Step 2: Check worktree**

Run:

```sh
git status --short
```

Expected: only intentional implementation files are modified or untracked.

- [ ] **Step 3: Commit any verification-only corrections**

If final verification required small compatibility corrections, commit them:

```sh
git add <corrected-files>
git commit -m "fix: align go lint compatibility"
```

Expected: clean commit containing only compatibility corrections.

- [ ] **Step 4: Record remaining deferred scope in final handoff**

Final handoff must mention:

```text
Implemented: awiki lint core engine, strict core compatibility, shell shim.
Verified: go test ./..., bats tests/lint_test.bats, bats tests/lint_synth_test.bats.
Deferred: synth/data/chart/task lint ports, qmd query workflow, ingest workflow, build/template workflows.
```

Expected: the user can see exactly what moved to Go and what still depends on legacy adapters.
