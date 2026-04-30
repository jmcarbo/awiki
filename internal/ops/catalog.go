package ops

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// UpdateCatalogOptions configures `awiki update-catalog`. Mirrors
// scripts/update-catalog.sh.
type UpdateCatalogOptions struct {
	RepoRoot   string // default cwd
	ContentDir string // default <repoRoot>/content
	Catalog    string // default <contentDir>/catalog.md
}

// catalogGroup ordering mirrors the OrderedDict in update-catalog.sh.
// Two type values map to the same group name; the type→group lookup
// keeps insertion order so unknown types fall through to "Misc".
var catalogGroupOrder = []struct{ typeName, group string }{
	{"entity", "Entities"},
	{"concept", "Concepts"},
	{"topic", "Topics"},
	{"source", "Sources"},
	{"synthesis", "Synthesis"},
	{"deck", "Synthesis"},
	{"chart", "Synthesis"},
	{"canvas", "Synthesis"},
}

// catalogSectionOrder is the bottom-of-file ordering. Bash:
//
//	("Entities","Concepts","Topics","Sources","Synthesis","Misc").
var catalogSectionOrder = []string{"Entities", "Concepts", "Topics", "Sources", "Synthesis", "Misc"}

// UpdateCatalog regenerates content/catalog.md (or the configured
// override). The bash form runs an embedded Python script; this Go
// port reproduces the same group/section logic byte-for-byte.
//
// Returns 0 on success, 1 if the catalog file is absent.
func UpdateCatalog(opts UpdateCatalogOptions, stdout, stderr io.Writer) int {
	repoRoot := opts.RepoRoot
	if repoRoot == "" {
		repoRoot, _ = os.Getwd()
	}
	contentDir := opts.ContentDir
	if contentDir == "" {
		contentDir = filepath.Join(repoRoot, "content")
	}
	catalog := opts.Catalog
	if catalog == "" {
		catalog = filepath.Join(contentDir, "catalog.md")
	}
	if _, err := os.Stat(catalog); err != nil {
		fmt.Fprintf(stderr, "no catalog at %s\n", catalog)
		return 1
	}

	typeToGroup := make(map[string]string, len(catalogGroupOrder))
	for _, e := range catalogGroupOrder {
		typeToGroup[e.typeName] = e.group
	}

	entries := make(map[string][]string)

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
		fmt.Fprintf(stderr, "update-catalog: walk: %v\n", err)
		return 1
	}
	// Bash uses `glob.glob(... recursive=True)` which returns paths in
	// directory-listing order; the Python `sorted()` call then puts them
	// in lexical order. Match.
	sort.Strings(paths)

	for _, path := range paths {
		slug := strings.TrimSuffix(filepath.Base(path), ".md")
		switch slug {
		case "_index", "catalog", "log":
			continue
		}
		fm, ok := readFrontmatterFlat(path)
		if !ok {
			continue
		}
		t := fm["type"]
		switch t {
		case "log", "catalog", "section-index":
			continue
		}
		section, known := typeToGroup[t]
		if !known {
			section = "Misc"
		}
		title := fm["title"]
		if title == "" {
			title = slug
		}
		entries[section] = append(entries[section], fmt.Sprintf("- [[%s]] — %s", slug, title))
	}

	// Read existing catalog body up to first `## ` (RE.split, maxsplit=1, MULTILINE).
	full, err := os.ReadFile(catalog)
	if err != nil {
		fmt.Fprintf(stderr, "update-catalog: read: %v\n", err)
		return 1
	}
	prelude := splitAtFirstHeading(string(full))
	prelude = strings.TrimRight(prelude, "\n") + "\n\n"

	var b strings.Builder
	b.WriteString(prelude)
	for _, section := range catalogSectionOrder {
		rows, ok := entries[section]
		if !ok || len(rows) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n", section)
		b.WriteString(strings.Join(rows, "\n"))
		b.WriteString("\n\n")
	}
	out := strings.TrimRight(b.String(), "\n") + "\n"
	if err := os.WriteFile(catalog, []byte(out), 0o644); err != nil {
		fmt.Fprintf(stderr, "update-catalog: write: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "CATALOG-OK")
	return 0
}

// UpdateCatalogCLI parses argv and dispatches.
func UpdateCatalogCLI(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintln(stderr, "usage: awiki update-catalog")
		return 2
	}
	return UpdateCatalog(UpdateCatalogOptions{}, stdout, stderr)
}

var firstHeadingRE = regexp.MustCompile(`(?m)^## `)

// splitAtFirstHeading mirrors Python's
// `re.split(r"^## ", full, maxsplit=1, flags=re.M)`. Returns prelude
// (everything before the first `^## `, exclusive). If there is no
// heading the entire input is returned.
func splitAtFirstHeading(s string) string {
	loc := firstHeadingRE.FindStringIndex(s)
	if loc == nil {
		return s
	}
	return s[:loc[0]]
}

// readFrontmatterFlat reads the leading `---` ... `---` frontmatter
// block of path and returns a flat key→value map. Mirrors the bash
// regex `re.match(r"---\n(.*?)\n---", text, re.S)` plus the loop
// that splits each `key: value` line.
func readFrontmatterFlat(path string) (map[string]string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	s := string(data)
	if !strings.HasPrefix(s, "---\n") {
		return nil, false
	}
	end := strings.Index(s[4:], "\n---")
	if end < 0 {
		return nil, false
	}
	body := s[4 : 4+end]
	out := make(map[string]string)
	for line := range strings.SplitSeq(body, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		v = strings.Trim(v, `"`)
		out[k] = v
	}
	return out, true
}
