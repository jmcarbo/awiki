package git

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// TransformOptions captures the argv shape the bash driver supplies to
// scripts/ingest-git-transform.py (lines 217-227). Each field maps 1:1
// to a `--<flag>` from the Python.
type TransformOptions struct {
	InPath       string
	OutPath      string
	RepoKey      string
	RepoName     string
	RepoRelpath  string
	GitURL       string
	GitBlobSHA   string
	AssetOutDir  string
	UpstreamRoot string
	Private      bool

	// SlugMap is the rel-path → slug mapping the Python reads from
	// stdin (ingest-git-transform.py:230). The driver computes it from
	// the working set of repo paths before iterating, so the
	// transformer can rewrite cross-file md links into wikilinks.
	SlugMap map[string]string

	// Today defaults to date.today() (UTC) in the Python; tests inject
	// a deterministic clock via this field.
	Today string
}

// TransformResult reports what the transformer wrote, including
// warnings emitted on stdout.
type TransformResult struct {
	OutPath   string
	Wrote     bool   // false when the body was <10 chars (skip case).
	Warnings  []string
	OutputLog string // verbatim "OK|0" or "WARN|skipped..." line
}

// upstreamFmRe matches a leading YAML frontmatter block. Mirrors the
// Python UPSTREAM_FM_RE: `^---\s*\n(.*?)\n---\s*\n` (DOTALL).
var upstreamFmRe = regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\s*\n`)

// stripUpstreamFrontmatter mirrors the Python helper: peel a leading
// `--- ... ---` block, returning a tiny meta dict {title:?} and the
// remaining body. Title parsing handles bare/single-quoted/double-quoted.
func stripUpstreamFrontmatter(text string) (map[string]string, string) {
	m := upstreamFmRe.FindStringSubmatchIndex(text)
	if m == nil {
		return map[string]string{}, text
	}
	raw := text[m[2]:m[3]]
	title := ""
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(line, " \t\r")
		if strings.HasPrefix(line, "title:") {
			v := strings.TrimSpace(line[len("title:"):])
			v = strings.Trim(v, "\"'")
			title = v
			break
		}
	}
	return map[string]string{"title": title}, text[m[1]:]
}

// deriveTitle: prefer upstream meta `title`, else first `# Heading`,
// else fallback. Mirrors the Python derive_title.
func deriveTitle(meta map[string]string, body, fallback string) string {
	if t := meta["title"]; t != "" {
		return t
	}
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(line[2:])
		}
	}
	return fallback
}

// emitFrontmatter mirrors the Python emit_frontmatter (lines 48-70).
// Field order is fixed (the Python ships an explicit list of strings);
// we emit the same lines in the same order.
func emitFrontmatter(opts TransformOptions, title, today string) string {
	tags := []string{"git", opts.RepoName}
	if opts.Private {
		tags = append(tags, "private")
	}
	tagsInline := "[" + strings.Join(tags, ", ") + "]"
	lines := []string{
		"---",
		// Python form: f'title: "{title}"' — no escaping. Mirroring
		// byte-for-byte means we must NOT use %q (which escapes). The
		// upstream meta strips quotes already, so embedded double-
		// quotes are extremely rare in practice, but if they slip
		// through they pass through unchanged here just like Python.
		fmt.Sprintf("title: \"%s\"", title),
		fmt.Sprintf("date: %s", today),
		fmt.Sprintf("last_updated: %s", today),
		"type: source",
		"provenance: git",
		fmt.Sprintf("git_repo: %s", opts.RepoName),
		fmt.Sprintf("git_path: %s", opts.RepoRelpath),
		fmt.Sprintf("git_blob_sha: %s", opts.GitBlobSHA),
		fmt.Sprintf("git_url: %s", opts.GitURL),
		fmt.Sprintf("tags: %s", tagsInline),
		"sources: []",
		"draft: false",
		"---",
		"",
	}
	return strings.Join(lines, "\n")
}

// normalizeRelpath mirrors the Python _normalize_relpath helper. Joins
// link_target (relative to current_relpath) into a repo-relpath, drops
// fragments + query strings, returns "" for absolute or escape paths.
func normalizeRelpath(currentRelpath, linkTarget string) string {
	if strings.Contains(linkTarget, "://") || strings.HasPrefix(linkTarget, "/") {
		return ""
	}
	target := linkTarget
	if i := strings.Index(target, "#"); i >= 0 {
		target = target[:i]
	}
	if i := strings.Index(target, "?"); i >= 0 {
		target = target[:i]
	}
	if target == "" {
		return ""
	}
	base := filepath.Dir(currentRelpath)
	var joined string
	if base != "" && base != "." {
		joined = filepath.Clean(filepath.Join(base, target))
	} else {
		joined = filepath.Clean(target)
	}
	joined = filepath.ToSlash(joined)
	if strings.HasPrefix(joined, "..") || joined == "." {
		return ""
	}
	return joined
}

// isMdTarget mirrors the Python _is_md_target helper.
func isMdTarget(p string) bool {
	lower := strings.ToLower(p)
	return strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".mdx")
}

// fenceLineSet returns a set of zero-based line indices inside fenced
// or indented code blocks. The Python uses markdown-it tokens; we do a
// scan for ``` fences (CommonMark backtick fences) and 4-space indent
// blocks. This is sufficient for the v1 transformer corpus — the
// fixtures we ship exercise both styles.
//
// Algorithm:
//
//   - Track fence open/close lines for sequences of >=3 backticks at
//     line start (after optional indent). Lines inside the fence
//     (exclusive of the fence open line, inclusive of close-line) are
//     marked as code.
//   - Outside fences, lines that are >=4-space indented and follow a
//     blank line begin an indented code block; the block continues
//     until the next blank line or non-indented line.
//
// CommonMark caveat: an indented code block cannot interrupt a
// paragraph. The v1 transformer corpus does not exercise that edge,
// and the Python markdown-it path is the oracle for any future
// disagreement.
func fenceLineSet(body string) map[int]bool {
	out := map[int]bool{}
	lines := strings.Split(body, "\n")
	inFence := false
	fenceMarker := ""
	prevBlank := true
	for i, raw := range lines {
		trimmedLeft := strings.TrimLeft(raw, " \t")
		// Fence detection — backtick or tilde, length >= 3.
		isFenceLine, marker := detectFence(trimmedLeft)
		if inFence {
			out[i] = true
			if isFenceLine && strings.HasPrefix(trimmedLeft, fenceMarker) {
				inFence = false
				fenceMarker = ""
			}
			prevBlank = false
			continue
		}
		if isFenceLine {
			out[i] = true
			inFence = true
			fenceMarker = marker
			prevBlank = false
			continue
		}
		// Indented code block: 4+ space indent at start, after a blank
		// line, and not blank itself.
		if prevBlank && len(raw) >= 4 && raw[0] == ' ' && raw[1] == ' ' && raw[2] == ' ' && raw[3] == ' ' && strings.TrimSpace(raw) != "" {
			// Mark as code; continue marking subsequent lines that
			// are blank or indented until a non-indented non-blank
			// line ends the block. We process inside the loop by
			// re-checking on subsequent iterations through prevBlank
			// gating only on the start; once started, continue while
			// the line is blank or 4-space indented.
			out[i] = true
			// Lookahead: extend the block forward.
			j := i + 1
			for j < len(lines) {
				ln := lines[j]
				if strings.TrimSpace(ln) == "" {
					out[j] = true
					j++
					continue
				}
				if len(ln) >= 4 && ln[0] == ' ' && ln[1] == ' ' && ln[2] == ' ' && ln[3] == ' ' {
					out[j] = true
					j++
					continue
				}
				break
			}
		}
		prevBlank = strings.TrimSpace(raw) == ""
	}
	return out
}

// detectFence checks if line is a CommonMark code fence open/close
// marker. Returns (isFence, marker), where marker is the run of
// backticks/tildes at line start (e.g. "```").
func detectFence(line string) (bool, string) {
	if len(line) < 3 {
		return false, ""
	}
	c := line[0]
	if c != '`' && c != '~' {
		return false, ""
	}
	n := 0
	for n < len(line) && line[n] == c {
		n++
	}
	if n < 3 {
		return false, ""
	}
	return true, strings.Repeat(string(c), n)
}

// linkRe / refRe / reflinkRe / imgRe mirror the Python's regex set.
var (
	linkRe    = regexp.MustCompile(`(?:^|[^\x60])\[([^\]]+)\]\(([^)]+)\)`)
	imgRe     = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
	refRe     = regexp.MustCompile(`^\[([^\]]+)\]:\s*(\S+)\s*$`)
	reflinkRe = regexp.MustCompile(`\[([^\]]+)\]\[([^\]]+)\]`)
)

// rewriteImages walks each non-fenced line and copies referenced image
// files from upstreamRoot into assetOutDir. Returns (newBody,
// warnings). Mirrors Python rewrite_images.
func rewriteImages(body, currentRelpath, upstreamRoot, assetOutDir string, fence map[int]bool) (string, []string) {
	if upstreamRoot == "" {
		return body, nil
	}
	warnings := []string{}
	lines := strings.Split(body, "\n")
	for i := range lines {
		if fence[i] {
			continue
		}
		lines[i] = imgRe.ReplaceAllStringFunc(lines[i], func(match string) string {
			m := imgRe.FindStringSubmatch(match)
			if m == nil {
				return match
			}
			alt, href := m[1], m[2]
			hrefPath := strings.Fields(href)[0]
			if strings.Contains(hrefPath, "://") || strings.HasPrefix(hrefPath, "/") {
				return match
			}
			target := normalizeRelpath(currentRelpath, hrefPath)
			if target == "" {
				return match
			}
			src := filepath.Join(upstreamRoot, target)
			info, err := os.Stat(src)
			if err != nil || info.IsDir() {
				warnings = append(warnings, fmt.Sprintf("missing image: %s", target))
				return match
			}
			dest := filepath.Join(assetOutDir, target)
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				warnings = append(warnings, fmt.Sprintf("mkdir asset: %v", err))
				return match
			}
			if err := copyFile(src, dest); err != nil {
				warnings = append(warnings, fmt.Sprintf("copy asset: %v", err))
				return match
			}
			rel := filepath.ToSlash(dest)
			if strings.HasPrefix(rel, "content/") {
				rel = rel[len("content/"):]
			}
			return fmt.Sprintf("![%s](%s)", alt, rel)
		})
	}
	return strings.Join(lines, "\n"), warnings
}

// rewriteWikilinks rewrites in-line and reference-style markdown links
// into `[[slug|text]]` wikilinks when the target lives in slugMap.
// Mirrors Python rewrite_wikilinks. Skips fence lines.
func rewriteWikilinks(body, currentRelpath string, slugMap map[string]string, fence map[int]bool) (string, []string) {
	lines := strings.Split(body, "\n")
	// First pass: collect reference definitions.
	refs := map[string]string{}
	for i, line := range lines {
		if fence[i] {
			continue
		}
		if m := refRe.FindStringSubmatch(line); m != nil {
			refs[strings.ToLower(m[1])] = m[2]
		}
	}
	warnings := []string{}
	for i, line := range lines {
		if fence[i] {
			continue
		}
		// Inline links via linkRe.
		out := linkRe.ReplaceAllStringFunc(line, func(match string) string {
			m := linkRe.FindStringSubmatch(match)
			if m == nil {
				return match
			}
			text, href := m[1], m[2]
			hrefPath := strings.Fields(href)[0]
			if !isMdTarget(hrefPath) {
				return match
			}
			target := normalizeRelpath(currentRelpath, hrefPath)
			slug, ok := slugMap[target]
			if !ok || slug == "" {
				return match
			}
			// Preserve the "non-backtick" prefix the regex captured.
			prefix := match[:len(match)-len(fmt.Sprintf("[%s](%s)", text, href))]
			return prefix + fmt.Sprintf("[[%s|%s]]", slug, text)
		})
		// Reference-style links via reflinkRe.
		out = reflinkRe.ReplaceAllStringFunc(out, func(match string) string {
			m := reflinkRe.FindStringSubmatch(match)
			if m == nil {
				return match
			}
			text, refID := m[1], strings.ToLower(m[2])
			href, ok := refs[refID]
			if !ok || !isMdTarget(href) {
				return match
			}
			target := normalizeRelpath(currentRelpath, href)
			slug, ok := slugMap[target]
			if !ok || slug == "" {
				return match
			}
			return fmt.Sprintf("[[%s|%s]]", slug, text)
		})
		lines[i] = out
	}
	return strings.Join(lines, "\n"), warnings
}

// Transform produces one awiki source page from one upstream markdown
// file. Mirrors scripts/ingest-git-transform.py:main.
//
// stdout/stderr write semantics:
//
//   - On success, prints "OK|0" on stdout (mirrors Python line 264).
//   - On a <10-char body, prints "WARN|skipped: <10 char body" and
//     returns Wrote=false (mirrors line 240-241).
//   - Per-image / per-link warnings print as "WARN|<msg>".
//
// Returns an error only on hard failure (input file missing, write
// error). The Python's "in not found" path returns exit 1.
func Transform(opts TransformOptions, stdout io.Writer) (TransformResult, error) {
	res := TransformResult{OutPath: opts.OutPath}

	info, err := os.Stat(opts.InPath)
	if err != nil || info.IsDir() {
		return res, fmt.Errorf("in not found: %s", opts.InPath)
	}
	raw, err := os.ReadFile(opts.InPath)
	if err != nil {
		return res, fmt.Errorf("read %s: %w", opts.InPath, err)
	}
	if len(strings.TrimSpace(string(raw))) < 10 {
		fmt.Fprintln(stdout, "WARN|skipped: <10 char body")
		res.OutputLog = "WARN|skipped: <10 char body"
		return res, nil
	}
	meta, body := stripUpstreamFrontmatter(string(raw))
	fallback := deriveFallbackTitle(opts.InPath)
	title := deriveTitle(meta, body, fallback)

	fence := fenceLineSet(body)
	if opts.UpstreamRoot != "" {
		var imgWarn []string
		body, imgWarn = rewriteImages(body, opts.RepoRelpath, opts.UpstreamRoot, opts.AssetOutDir, fence)
		for _, w := range imgWarn {
			fmt.Fprintf(stdout, "WARN|%s\n", w)
			res.Warnings = append(res.Warnings, w)
		}
	}
	body, linkWarn := rewriteWikilinks(body, opts.RepoRelpath, opts.SlugMap, fence)
	for _, w := range linkWarn {
		fmt.Fprintf(stdout, "WARN|%s\n", w)
		res.Warnings = append(res.Warnings, w)
	}

	today := opts.Today
	if today == "" {
		today = time.Now().UTC().Format("2006-01-02")
	}
	outText := emitFrontmatter(opts, title, today) + strings.TrimLeft(body, "\n")

	if err := os.MkdirAll(filepath.Dir(opts.OutPath), 0o755); err != nil {
		return res, fmt.Errorf("mkdir out dir: %w", err)
	}
	tmp := fmt.Sprintf("%s.tmp.%d", opts.OutPath, os.Getpid())
	if err := os.WriteFile(tmp, []byte(outText), 0o644); err != nil {
		return res, fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, opts.OutPath); err != nil {
		_ = os.Remove(tmp)
		return res, fmt.Errorf("rename: %w", err)
	}

	fmt.Fprintln(stdout, "OK|0")
	res.OutputLog = "OK|0"
	res.Wrote = true
	return res, nil
}

// deriveFallbackTitle mirrors `Path(in_path).stem.replace("-", " ").title()`.
// Python `str.title()` capitalizes the first letter of every word
// (whitespace-delimited). We mirror that by walking runes.
func deriveFallbackTitle(inPath string) string {
	stem := filepath.Base(inPath)
	if i := strings.LastIndex(stem, "."); i >= 0 {
		stem = stem[:i]
	}
	stem = strings.ReplaceAll(stem, "-", " ")
	return titleCase(stem)
}

func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if w == "" {
			continue
		}
		runes := []rune(w)
		runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
		for j := 1; j < len(runes); j++ {
			runes[j] = []rune(strings.ToLower(string(runes[j])))[0]
		}
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

// copyFile is a small file copier. Defined locally to avoid a dependency
// on the formats package's copyFile.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}
