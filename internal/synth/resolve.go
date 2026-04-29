package synth

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// synthDir returns the synthesis pages directory (mirrors SYNTH_DIR in bash).
// The bash default is content/synthesis.
func (r *Runner) synthDir() string {
	if d := os.Getenv("AWIKI_SYNTH_DIR"); d != "" {
		return filepath.Join(r.RepoRoot, d)
	}
	return filepath.Join(r.ContentDir, "synthesis")
}

// Resolve reads <synthDir>/<slug>.md, parses scope, resolves to a slug
// list, applies all filters, and emits one slug per line ASCII-sorted to out.
// Matches scripts/synth.sh:cmd_resolve + synth_resolve_to_slugs output.
func (r *Runner) Resolve(ctx context.Context, slug string, out io.Writer) error {
	synthPagePath := filepath.Join(r.synthDir(), slug+".md")
	data, err := os.ReadFile(synthPagePath)
	if err != nil {
		return err
	}

	// Extract raw frontmatter text.
	fmText := extractFrontmatterText(string(data))

	// Parse scope block.
	scope := ParseScopeFromFrontmatter(fmText)

	// Determine privacy of the synth page itself.
	synthPrivate := isPrivatePath(synthPagePath)

	// Resolve primary slug list.
	slugs, err := r.resolveScope(ctx, scope, synthPrivate)
	if err != nil {
		return err
	}

	// Emit one slug per line, ASCII-sorted (sort.Strings is ASCII/lexicographic).
	sort.Strings(slugs)
	for _, s := range slugs {
		fmt.Fprintln(out, s)
	}
	return nil
}

// resolveScope dispatches on scope.Kind and applies all post-filters.
func (r *Runner) resolveScope(ctx context.Context, scope Scope, synthPrivate bool) ([]string, error) {
	var primary []string
	var err error

	switch scope.Kind {
	case "slugs":
		primary = append([]string(nil), scope.Slugs...)
	case "tag":
		primary, err = r.collectByTag(scope.Tag)
		if err != nil {
			return nil, err
		}
	case "query":
		if r.Qmd == nil {
			return nil, fmt.Errorf("scope.kind=query but qmd adapter unavailable")
		}
		out, _, qerr := r.Qmd.Search(ctx, r.RepoRoot, scope.Query)
		if qerr != nil {
			return nil, fmt.Errorf("qmd search: %w", qerr)
		}
		primary = parseQmdOutput(out)
	default:
		return nil, fmt.Errorf("scope must include exactly one of: tag, slugs, query")
	}

	// Apply filters in order, mirroring synth_resolve_to_slugs.

	// Filter: exclude_tags
	if len(scope.ExcludeTags) > 0 {
		primary, err = r.filterExcludeTags(primary, scope.ExcludeTags)
		if err != nil {
			return nil, err
		}
	}

	// Filter: min_last_updated
	if scope.MinLastUpdated != "" {
		primary, err = r.filterMinLastUpdated(primary, scope.MinLastUpdated)
		if err != nil {
			return nil, err
		}
	}

	// Filter: types
	if len(scope.Types) > 0 {
		primary, err = r.filterTypes(primary, scope.Types)
		if err != nil {
			return nil, err
		}
	}

	// Privacy filter: drop private pages unless synth page is itself private.
	if !synthPrivate {
		primary, err = r.filterPrivacy(primary)
		if err != nil {
			return nil, err
		}
	}

	// Dedupe (sort -u).
	return uniqueSorted(primary), nil
}

// collectByTag walks ContentDir and returns slugs of pages whose tags
// include tag. Mirrors bash synth_resolve_to_slugs tag branch.
func (r *Runner) collectByTag(tag string) ([]string, error) {
	var slugs []string
	err := filepath.WalkDir(r.ContentDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		tags, ferr := fmTags(path)
		if ferr != nil {
			return nil // skip unreadable pages
		}
		for _, t := range tags {
			if t == tag {
				slug := strings.TrimSuffix(filepath.Base(path), ".md")
				slugs = append(slugs, slug)
				break
			}
		}
		return nil
	})
	return slugs, err
}

// filterExcludeTags drops slugs whose page has any of the excluded tags.
func (r *Runner) filterExcludeTags(slugs, excludeTags []string) ([]string, error) {
	exSet := make(map[string]bool, len(excludeTags))
	for _, t := range excludeTags {
		exSet[t] = true
	}
	var out []string
	for _, slug := range slugs {
		path := r.slugToPath(slug)
		if path == "" {
			continue // slug not found; skip (mirrors bash behavior)
		}
		tags, _ := fmTags(path)
		skip := false
		for _, t := range tags {
			if exSet[t] {
				skip = true
				break
			}
		}
		if !skip {
			out = append(out, slug)
		}
	}
	return out, nil
}

// filterMinLastUpdated drops slugs whose page last_updated < minDate (lex compare).
func (r *Runner) filterMinLastUpdated(slugs []string, minDate string) ([]string, error) {
	var out []string
	for _, slug := range slugs {
		path := r.slugToPath(slug)
		if path == "" {
			continue
		}
		lu := fmField(path, "last_updated")
		if lu >= minDate {
			out = append(out, slug)
		}
	}
	return out, nil
}

// filterTypes keeps only slugs whose page type: is in the allowed list.
func (r *Runner) filterTypes(slugs, types []string) ([]string, error) {
	typeSet := make(map[string]bool, len(types))
	for _, t := range types {
		typeSet[t] = true
	}
	var out []string
	for _, slug := range slugs {
		path := r.slugToPath(slug)
		if path == "" {
			continue
		}
		t := fmField(path, "type")
		if typeSet[t] {
			out = append(out, slug)
		}
	}
	return out, nil
}

// filterPrivacy drops slugs under content/private/ (pages tagged "private"
// are not dropped here — the bash checks the path, not the tag, for privacy
// in synth_resolve_to_slugs).
//
// NOTE: The bash privacy filter checks: if SCOPE_TARGET_PRIVATE != 1, it
// calls synth_slug_to_path and checks if " $tags " contains " private ".
// So it checks the *private tag*, not the path. Let's match bash exactly.
func (r *Runner) filterPrivacy(slugs []string) ([]string, error) {
	var out []string
	for _, slug := range slugs {
		path := r.slugToPath(slug)
		if path == "" {
			continue
		}
		tags, _ := fmTags(path)
		hasPrivate := false
		for _, t := range tags {
			if t == "private" {
				hasPrivate = true
				break
			}
		}
		if !hasPrivate {
			out = append(out, slug)
		}
	}
	return out, nil
}

// slugToPath resolves a slug to an absolute file path. It first consults
// the slug-to-path map (.awiki/maps/slug-to-path.tsv), then falls back to
// searching ContentDir. Mirrors scripts/synth.sh:synth_slug_to_path.
func (r *Runner) slugToPath(slug string) string {
	mapPath := filepath.Join(r.RepoRoot, ".awiki", "maps", "slug-to-path.tsv")
	if f, err := os.Open(mapPath); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			parts := strings.SplitN(line, "\t", 2)
			if len(parts) == 2 && parts[0] == slug {
				rel := parts[1]
				return filepath.Join(r.ContentDir, rel)
			}
		}
	}
	// Fallback: find slug.md under ContentDir.
	var found string
	_ = filepath.WalkDir(r.ContentDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if filepath.Base(path) == slug+".md" {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// isPrivatePath reports whether the path is under a private/ directory.
func isPrivatePath(path string) bool {
	return strings.Contains(filepath.ToSlash(path), "/private/")
}

// extractFrontmatterText extracts the text between the first pair of "---"
// markers (not including the markers themselves).
func extractFrontmatterText(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(content, "---\n") {
		return ""
	}
	rest := content[4:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return rest
	}
	return rest[:end]
}

// fmTags reads the tags: field from a page file, returning a slice.
// Mirrors scripts/synth.sh:synth_fm_tags: strips [ ], quotes, and splits on comma/space.
func fmTags(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	raw := fmFieldFromText(extractFrontmatterText(string(data)), "tags")
	if raw == "" {
		return nil, nil
	}
	// Strip surrounding brackets.
	raw = strings.TrimPrefix(raw, "[")
	raw = strings.TrimSuffix(raw, "]")
	// Split on comma and trim.
	parts := strings.Split(raw, ",")
	var tags []string
	for _, p := range parts {
		t := strings.Trim(strings.TrimSpace(p), `"'`)
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags, nil
}

// fmField reads a single scalar field from a page's frontmatter.
// Mirrors scripts/synth.sh:synth_fm_field.
func fmField(path, field string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return fmFieldFromText(extractFrontmatterText(string(data)), field)
}

// fmFieldFromText extracts a scalar value for field from frontmatter text.
func fmFieldFromText(fmText, field string) string {
	for _, line := range strings.Split(fmText, "\n") {
		// Match "^<field>: <value>" at the top level (not indented).
		if strings.HasPrefix(line, field+":") {
			val := strings.TrimSpace(line[len(field)+1:])
			return val
		}
	}
	return ""
}

// slugRegexp matches valid wiki slug format.
var slugRegexp = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// parseQmdOutput parses qmd search output: first whitespace token of each
// non-empty line, filtered to valid slug pattern.
// Mirrors bash: qmd search ... | awk '{print $1}' | grep -E "$SLUG_REGEX"
func parseQmdOutput(text string) []string {
	var slugs []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		slug := fields[0]
		if slugRegexp.MatchString(slug) {
			slugs = append(slugs, slug)
		}
	}
	return slugs
}

// uniqueSorted deduplicates and sorts a slice of strings.
func uniqueSorted(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
