package ops

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"awiki/internal/adapters"
)

// BuildOptions configures `awiki build`. Mirrors scripts/build.sh.
type BuildOptions struct {
	RepoRoot string
	// MapsOnly: write the three slug/alias/title maps and exit (matches
	// `--maps-only` flag in bash).
	MapsOnly bool
	// Full: also render charts/queries (when AWIKI_DATA_LAYER=on) and
	// shell `hugo --minify --destination public` (matches `--full`).
	Full bool
	// HugoRunner is the adapter that exec's `hugo`. nil falls back to
	// adapters.ExecRunner{} when Full == true.
	HugoRunner adapters.Runner
}

// Build mirrors scripts/build.sh:
//
//  1. write .awiki/maps/{slug-to-path,alias-to-slug,slug-to-title}.tsv
//  2. with --maps-only, exit (BUILD-OK|maps-only=1).
//  3. otherwise rewrite every .md page into .awiki/build-content/, expanding
//     `[[wikilinks]]` to `[Title](/section/slug/)` (skip fenced + inline code).
//  4. with --full, also render charts/queries + shell hugo.
func Build(opts BuildOptions, stdout, stderr io.Writer) int {
	if opts.RepoRoot == "" {
		opts.RepoRoot, _ = os.Getwd()
	}

	contentDir := filepath.Join(opts.RepoRoot, "content")
	mapsDir := filepath.Join(opts.RepoRoot, ".awiki", "maps")
	buildDir := filepath.Join(opts.RepoRoot, ".awiki", "build-content")

	if err := os.MkdirAll(mapsDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "build: %v\n", err)
		return 1
	}
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "build: %v\n", err)
		return 1
	}

	pages, err := scanContentPages(contentDir)
	if err != nil {
		fmt.Fprintf(stderr, "build: %v\n", err)
		return 1
	}
	slugMap, aliasMap, titleMap := buildMaps(pages)

	if err := writeMap(filepath.Join(mapsDir, "slug-to-path.tsv"), slugMap); err != nil {
		fmt.Fprintf(stderr, "build: %v\n", err)
		return 1
	}
	if err := writeMap(filepath.Join(mapsDir, "alias-to-slug.tsv"), aliasMap); err != nil {
		fmt.Fprintf(stderr, "build: %v\n", err)
		return 1
	}
	if err := writeMap(filepath.Join(mapsDir, "slug-to-title.tsv"), titleMap); err != nil {
		fmt.Fprintf(stderr, "build: %v\n", err)
		return 1
	}

	if opts.MapsOnly {
		fmt.Fprintln(stdout, "BUILD-OK|maps-only=1")
		return 0
	}

	// Atomic rebuild: write to BUILD_DIR.tmp, rename old aside, rename tmp -> BUILD_DIR.
	tmpDir := buildDir + ".tmp"
	_ = os.RemoveAll(tmpDir)
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "build: %v\n", err)
		return 1
	}

	resolver := newWikilinkResolver(slugMap, aliasMap, titleMap)
	for _, page := range pages {
		src := filepath.Join(contentDir, page.relPath)
		dst := filepath.Join(tmpDir, page.relPath)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			fmt.Fprintf(stderr, "build: %v\n", err)
			return 1
		}
		body, err := os.ReadFile(src)
		if err != nil {
			fmt.Fprintf(stderr, "build: %v\n", err)
			return 1
		}
		out := rewriteOutsideCode(string(body), resolver)
		if err := os.WriteFile(dst, []byte(out), 0o644); err != nil {
			fmt.Fprintf(stderr, "build: %v\n", err)
			return 1
		}
	}
	// Atomic swap.
	oldDir := fmt.Sprintf("%s.old.%d", buildDir, time.Now().UnixNano())
	if _, err := os.Stat(buildDir); err == nil {
		if err := os.Rename(buildDir, oldDir); err != nil {
			fmt.Fprintf(stderr, "build: %v\n", err)
			return 1
		}
	}
	if err := os.Rename(tmpDir, buildDir); err != nil {
		fmt.Fprintf(stderr, "build: %v\n", err)
		return 1
	}
	_ = os.RemoveAll(oldDir)

	fmt.Fprintf(stdout, "BUILD-OK|content=content|build=.awiki/build-content\n")

	if opts.Full {
		// Chart + query render (best effort; mirrors bash || warn pattern).
		if cfgEnabled(opts.RepoRoot, "AWIKI_DATA_LAYER", "on") {
			runner := opts.HugoRunner
			if runner == nil {
				runner = adapters.ExecRunner{}
			}
			runOrWarn(runner, opts.RepoRoot, []string{"awiki", "query", "render"}, "query-render", stdout)
			runOrWarn(runner, opts.RepoRoot, []string{"awiki", "query", "fence-render"}, "query-fence-render", stdout)
			runOrWarn(runner, opts.RepoRoot, []string{"awiki", "chart", "render"}, "charts-render", stdout)
		}
		runner := opts.HugoRunner
		if runner == nil {
			runner = adapters.ExecRunner{}
		}
		out, code, err := runner.RunInDir(nil, opts.RepoRoot, "hugo", "--minify", "--destination", "public")
		if err != nil || code != 0 {
			fmt.Fprintf(stderr, "build: hugo: %v\n%s", err, out)
			return 1
		}
		fmt.Fprintln(stdout, "HUGO-OK|out=public")
	}
	return 0
}

// runOrWarn matches `bash scripts/query.sh ... || echo "BUILD|WARN|..."`.
// The actual runner is wrapped — ignore errors but record a structured warning.
func runOrWarn(runner adapters.Runner, dir string, argv []string, label string, stdout io.Writer) {
	out, code, err := runner.RunInDir(nil, dir, argv[0], argv[1:]...)
	if err != nil || code != 0 {
		fmt.Fprintf(stdout, "BUILD|WARN|%s returned non-zero\n", label)
	}
	_ = out
}

// cfgEnabled reads .awiki/config and returns true if KEY=val is set.
func cfgEnabled(repoRoot, key, want string) bool {
	data, err := os.ReadFile(filepath.Join(repoRoot, ".awiki", "config"))
	if err != nil {
		return false
	}
	prefix := key + "="
	for _, line := range strings.Split(string(data), "\n") {
		rest, ok := strings.CutPrefix(line, prefix)
		if !ok {
			continue
		}
		if strings.TrimSpace(rest) == want {
			return true
		}
	}
	return false
}

// === content scan + maps ===

type contentPage struct {
	relPath string // relative to content/, e.g. "entities/foo.md"
	slug    string // basename minus .md
	title   string
	aliases []string
}

// scanContentPages walks contentDir and parses each page's title + aliases.
// Skips _index.md files (matching scripts/build.sh).
func scanContentPages(contentDir string) ([]contentPage, error) {
	var pages []contentPage
	err := filepath.Walk(contentDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		name := info.Name()
		if !strings.HasSuffix(name, ".md") {
			return nil
		}
		if name == "_index.md" {
			return nil
		}
		rel, _ := filepath.Rel(contentDir, p)
		body, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		title, aliases := parseBuildFrontmatter(string(body))
		pages = append(pages, contentPage{
			relPath: rel,
			slug:    strings.TrimSuffix(name, ".md"),
			title:   title,
			aliases: aliases,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].relPath < pages[j].relPath })
	return pages, nil
}

// buildMaps converts pages into TSV-ready slug→path, alias→slug, slug→title.
// Each map is a slice of "k\tv" strings preserving insertion order.
func buildMaps(pages []contentPage) (slugMap, aliasMap, titleMap []string) {
	for _, p := range pages {
		slugMap = append(slugMap, p.slug+"\t"+p.relPath)
		if p.title != "" {
			titleMap = append(titleMap, p.slug+"\t"+p.title)
		}
		for _, a := range p.aliases {
			aliasMap = append(aliasMap, a+"\t"+p.slug)
		}
	}
	return
}

func writeMap(path string, lines []string) error {
	var b strings.Builder
	for _, line := range lines {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// parseBuildFrontmatter extracts only `title` and `aliases:` from a YAML
// frontmatter block. Mirrors the fallback parser in scripts/build.sh
// (state-machine handling of single+double quotes and escaped quotes).
func parseBuildFrontmatter(text string) (string, []string) {
	front, ok := extractFrontmatter(text)
	if !ok {
		return "", nil
	}
	var title string
	var aliases []string
	for _, line := range strings.Split(front, "\n") {
		switch {
		case strings.HasPrefix(line, "title:"):
			title = parseScalarValue(strings.TrimSpace(strings.TrimPrefix(line, "title:")))
		case strings.HasPrefix(line, "aliases:"):
			rest := strings.TrimSpace(strings.TrimPrefix(line, "aliases:"))
			if strings.HasPrefix(rest, "[") {
				aliases = parseFlowList(rest)
			}
		}
	}
	return title, aliases
}

// extractFrontmatter returns the body between the leading `---` markers.
func extractFrontmatter(text string) (string, bool) {
	if !strings.HasPrefix(text, "---") {
		return "", false
	}
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", false
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n"), true
		}
	}
	return "", false
}

// parseScalarValue strips a single layer of YAML quoting from a scalar.
func parseScalarValue(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		inner := v[1 : len(v)-1]
		var b strings.Builder
		for j := 0; j < len(inner); j++ {
			if inner[j] == '\\' && j+1 < len(inner) {
				b.WriteByte(inner[j+1])
				j++
				continue
			}
			b.WriteByte(inner[j])
		}
		return b.String()
	}
	if len(v) >= 2 && v[0] == '\'' && v[len(v)-1] == '\'' {
		return strings.ReplaceAll(v[1:len(v)-1], "''", "'")
	}
	return v
}

// parseFlowList parses a YAML flow-style list `[a, "b", 'c, with comma']`.
// Honours single/double quotes and backslash escapes (mirrors scripts/build.sh).
func parseFlowList(s string) []string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return nil
	}
	inner := s[1 : len(s)-1]
	var items []string
	var buf []byte
	state := "top"
	for i := 0; i < len(inner); i++ {
		c := inner[i]
		switch state {
		case "top":
			switch c {
			case ',':
				items = append(items, strings.TrimSpace(string(buf)))
				buf = buf[:0]
			case '"':
				state = "dq"
				buf = append(buf, c)
			case '\'':
				state = "sq"
				buf = append(buf, c)
			default:
				buf = append(buf, c)
			}
		case "dq":
			buf = append(buf, c)
			if c == '\\' && i+1 < len(inner) {
				buf = append(buf, inner[i+1])
				i++
				continue
			}
			if c == '"' {
				state = "top"
			}
		case "sq":
			buf = append(buf, c)
			if c == '\'' {
				if i+1 < len(inner) && inner[i+1] == '\'' {
					buf = append(buf, inner[i+1])
					i++
					continue
				}
				state = "top"
			}
		}
	}
	if tail := strings.TrimSpace(string(buf)); tail != "" {
		items = append(items, tail)
	}
	out := make([]string, 0, len(items))
	for _, raw := range items {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		out = append(out, parseScalarValue(v))
	}
	return out
}

// === wikilink rewrite ===

type wikilinkResolver struct {
	slugs   map[string]string // slug → relPath
	aliases map[string]string // alias → slug
	titles  map[string]string // slug → title
}

func newWikilinkResolver(slugMap, aliasMap, titleMap []string) *wikilinkResolver {
	r := &wikilinkResolver{
		slugs:   map[string]string{},
		aliases: map[string]string{},
		titles:  map[string]string{},
	}
	for _, line := range slugMap {
		k, v, ok := strings.Cut(line, "\t")
		if ok {
			r.slugs[k] = v
		}
	}
	for _, line := range aliasMap {
		k, v, ok := strings.Cut(line, "\t")
		if ok {
			r.aliases[k] = v
		}
	}
	for _, line := range titleMap {
		k, v, ok := strings.Cut(line, "\t")
		if ok {
			r.titles[k] = v
		}
	}
	return r
}

func (r *wikilinkResolver) resolve(inner string) (replacement string, ok bool) {
	target := inner
	display := inner
	if i := strings.Index(inner, "|"); i >= 0 {
		target = inner[:i]
		display = inner[i+1:]
	}
	rel, hit := r.slugs[target]
	resolvedSlug := target
	if !hit {
		alias, aok := r.aliases[target]
		if aok {
			resolvedSlug = alias
			rel, hit = r.slugs[alias]
		}
	}
	if !hit {
		return "", false
	}
	section, _ := filepath.Split(rel)
	section = strings.TrimSuffix(section, "/")
	if display == target {
		if t, ok := r.titles[resolvedSlug]; ok {
			display = t
		} else {
			display = target
		}
	}
	base := strings.TrimSuffix(filepath.Base(rel), ".md")
	url := "/"
	if section != "" {
		url += section + "/"
	}
	url += base + "/"
	return fmt.Sprintf("[%s](%s)", display, url), true
}

var buildWikilinkRE = regexp.MustCompile(`\[\[([^\]]+)\]\]`)

func substituteWikilinks(text string, r *wikilinkResolver) string {
	return wikilinkRE.ReplaceAllStringFunc(text, func(m string) string {
		inner := m[2 : len(m)-2]
		if rep, ok := r.resolve(inner); ok {
			return rep
		}
		return m
	})
}

var fenceRE = regexp.MustCompile("^(\\s*)(`{3,}|~{3,})(.*)$")

// rewriteOutsideCode applies wikilink substitution while skipping fenced
// code blocks and inline `code` spans. Mirrors scripts/build.sh's
// rewrite_outside_code/rewrite_inline.
func rewriteOutsideCode(s string, r *wikilinkResolver) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	inFence := false
	fenceChar := byte(0)
	fenceLen := 0
	for _, line := range lines {
		if m := fenceRE.FindStringSubmatch(line); m != nil {
			if !inFence {
				inFence = true
				fenceChar = m[2][0]
				fenceLen = len(m[2])
				out = append(out, line)
				continue
			}
			// closing fence?
			if m[2][0] == fenceChar && len(m[2]) >= fenceLen && strings.TrimSpace(m[3]) == "" {
				inFence = false
			}
			out = append(out, line)
			continue
		}
		if inFence {
			out = append(out, line)
			continue
		}
		out = append(out, rewriteInline(line, r))
	}
	return strings.Join(out, "\n")
}

// rewriteInline walks <line>, skipping balanced runs of backticks, and
// substitutes wikilinks in the surviving spans.
func rewriteInline(line string, r *wikilinkResolver) string {
	var parts []string
	var buf []byte
	i := 0
	for i < len(line) {
		if line[i] == '`' {
			if len(buf) > 0 {
				parts = append(parts, substituteWikilinks(string(buf), r))
				buf = buf[:0]
			}
			j := i
			for j < len(line) && line[j] == '`' {
				j++
			}
			runLen := j - i
			tick := strings.Repeat("`", runLen)
			closeIdx := strings.Index(line[j:], tick)
			if closeIdx == -1 {
				// unmatched: append remainder as literal.
				parts = append(parts, line[i:])
				i = len(line)
				break
			}
			parts = append(parts, line[i:j+closeIdx+runLen])
			i = j + closeIdx + runLen
			continue
		}
		buf = append(buf, line[i])
		i++
	}
	if len(buf) > 0 {
		parts = append(parts, substituteWikilinks(string(buf), r))
	}
	return strings.Join(parts, "")
}

// BuildCLI parses argv and dispatches Build.
func BuildCLI(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	fs.SetOutput(stderr)
	mapsOnly := fs.Bool("maps-only", false, "write maps and exit")
	full := fs.Bool("full", false, "render charts/queries + run hugo")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected positional: %s\n", fs.Arg(0))
		return 1
	}
	if *mapsOnly && *full {
		fmt.Fprintln(stderr, "--maps-only and --full are mutually exclusive")
		return 1
	}
	repoRoot := os.Getenv("AWIKI_REPO_ROOT")
	if repoRoot == "" {
		repoRoot, _ = os.Getwd()
	}
	return Build(BuildOptions{
		RepoRoot: repoRoot,
		MapsOnly: *mapsOnly,
		Full:     *full,
	}, stdout, stderr)
}
