package chart

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"awiki/internal/wiki"
)

var dataNameRE = regexp.MustCompile(`^\[\[([a-z0-9][a-z0-9-]*)\]\]$`)

func Run(opts Options) ([]Diagnostic, []FixRecord, error) {
	pages, err := chartPages(opts)
	if err != nil {
		return nil, nil, err
	}
	pageBySlug := map[string]wiki.Page{}
	for _, page := range pages {
		pageBySlug[page.Slug] = page
	}
	var diagnostics []Diagnostic
	var fixes []FixRecord
	refCounts := map[string]int{}
	for _, page := range pages {
		pageDiagnostics, pageFixes, pageRefs := lintPage(page, opts)
		diagnostics = append(diagnostics, pageDiagnostics...)
		fixes = append(fixes, pageFixes...)
		for slug, count := range pageRefs {
			refCounts[slug] += count
		}
	}
	for _, diagnostic := range lintAggregate(refCounts, opts) {
		diagnostics = append(diagnostics, diagnostic)
	}
	sortDiagnostics(diagnostics)
	return diagnostics, fixes, nil
}

func chartPages(opts Options) ([]wiki.Page, error) {
	var paths []string
	if opts.OnlyFile != "" {
		paths = append(paths, resolveOnlyFile(opts))
	} else if err := filepath.WalkDir(opts.ContentDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && filepath.Ext(path) == ".md" {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Strings(paths)
	pages := make([]wiki.Page, 0, len(paths))
	for _, path := range paths {
		page, err := wiki.ParsePage(path, opts.ContentDir)
		if err != nil {
			return nil, err
		}
		pages = append(pages, page)
	}
	return pages, nil
}

func resolveOnlyFile(opts Options) string {
	if filepath.IsAbs(opts.OnlyFile) {
		return opts.OnlyFile
	}
	if strings.HasPrefix(filepath.ToSlash(opts.OnlyFile), "content/") {
		root := opts.RepoRoot
		if root == "" {
			root = filepath.Dir(opts.ContentDir)
		}
		return filepath.Join(root, filepath.FromSlash(opts.OnlyFile))
	}
	return filepath.Join(opts.ContentDir, filepath.FromSlash(opts.OnlyFile))
}

func lintPage(page wiki.Page, opts Options) ([]Diagnostic, []FixRecord, map[string]int) {
	file := "content/" + page.RelPath
	var diagnostics []Diagnostic
	var fixes []FixRecord
	refs := map[string]int{}
	fences := extractVegaLiteFences(page.Body)
	if page.Frontmatter["chart_engine"] == "vega-lite" && len(fences) == 0 {
		diagnostics = append(diagnostics, diag(Error, file, "C5", "chart_engine=vega-lite but no vega-lite fence"))
	}

	for idx, fence := range fences {
		chartID := page.Slug
		if page.Type != "chart" || len(fences) != 1 {
			chartID = fmt.Sprintf("%s-fig%d", page.Slug, idx)
		}
		var spec any
		if err := json.Unmarshal([]byte(fence), &spec); err != nil {
			diagnostics = append(diagnostics, diag(Error, file, "C1", "fence body fails JSON parse (chart="+chartID+")"))
			continue
		}
		specMap, _ := spec.(map[string]any)
		if !hasVisualKey(specMap) {
			diagnostics = append(diagnostics, diag(Error, file, "C2", "spec missing mark/layer/hconcat/vconcat/facet/repeat (chart="+chartID+")"))
		}
		names := collectDataNames(spec)
		for _, slug := range names {
			refs[slug]++
			if reason := resolveDataset(slug, opts); reason != "" {
				diagnostics = append(diagnostics, diag(Error, file, "C3", fmt.Sprintf("RESOLVE|%s|%s|%s", page.Path, slug, reason)))
			}
		}
		for _, msg := range lintFields(spec, names, opts, chartID) {
			diagnostics = append(diagnostics, diag(Error, file, "C4", msg))
		}
		sidecar := filepath.Join(opts.RepoRoot, "assets", "charts", chartID+".svg")
		if _, err := os.Stat(sidecar); err != nil {
			diagnostics = append(diagnostics, diag(Warn, file, "C7", "sidecar missing for "+chartID+" (run charts-render)"))
		}
		for _, slug := range names {
			if datasetIsPrivate(slug, opts) && !pageIsPrivate(page) {
				diagnostics = append(diagnostics, diag(Error, file, "C-PRIV", fmt.Sprintf("chart references private dataset [[%s]] from non-private page (chart=%s)", slug, chartID)))
			}
		}
	}

	if page.Type == "chart" || strings.HasPrefix(page.RelPath, "charts/") {
		declared := page.Frontmatter["chart_data"]
		if declared != "" && len(fences) > 0 {
			var spec map[string]any
			if json.Unmarshal([]byte(fences[0]), &spec) == nil {
				if body := firstDataName(spec); body != "" && body != declared {
					if opts.Fix {
						if replaceFrontmatterScalar(page.Path, "chart_data", `"`+body+`"`) == nil {
							fixes = append(fixes, FixRecord{File: file, Message: fmt.Sprintf("chart_data: %s -> %s", declared, body)})
						}
					} else {
						diagnostics = append(diagnostics, diag(Warn, file, "C6", fmt.Sprintf("chart_data %s disagrees with body %s", declared, body)))
					}
				}
			}
		}
	}
	for _, edit := range chartPreviewEdits(page) {
		diagnostics = append(diagnostics, diag(Warn, file, "C9", edit))
	}
	return diagnostics, fixes, refs
}

func lintAggregate(refCounts map[string]int, opts Options) []Diagnostic {
	var out []Diagnostic
	for slug, count := range refCounts {
		if count > 5 {
			out = append(out, diag(Info, "content/datasets/"+slug+".md", "C8", fmt.Sprintf("referenced %d times across charts (consider type:chart page)", count)))
		}
	}
	return out
}

func extractVegaLiteFences(body string) []string {
	re := regexp.MustCompile("(?s)```vega-lite\\s*\\n(.*?)\\n```")
	matches := re.FindAllStringSubmatch(body, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		out = append(out, match[1])
	}
	return out
}

func hasVisualKey(spec map[string]any) bool {
	for _, key := range []string{"mark", "layer", "hconcat", "vconcat", "facet", "repeat"} {
		if _, ok := spec[key]; ok {
			return true
		}
	}
	return false
}

func collectDataNames(node any) []string {
	var out []string
	var walk func(any)
	walk = func(n any) {
		switch typed := n.(type) {
		case map[string]any:
			if data, ok := typed["data"].(map[string]any); ok {
				if name, ok := data["name"].(string); ok {
					if match := dataNameRE.FindStringSubmatch(name); match != nil {
						out = append(out, match[1])
					}
				}
			}
			for _, value := range typed {
				walk(value)
			}
		case []any:
			for _, value := range typed {
				walk(value)
			}
		}
	}
	walk(node)
	return out
}

func firstDataName(spec map[string]any) string {
	if data, ok := spec["data"].(map[string]any); ok {
		if name, ok := data["name"].(string); ok {
			return name
		}
	}
	return ""
}

func resolveDataset(slug string, opts Options) string {
	path := filepath.Join(opts.ContentDir, "datasets", slug+".md")
	page, err := wiki.ParsePage(path, opts.ContentDir)
	if err != nil {
		return "not_found"
	}
	if page.Type != "dataset" {
		return "not_a_dataset"
	}
	switch page.Frontmatter["storage"] {
	case "file", "inline":
		return ""
	default:
		return "bad_storage"
	}
}

func lintFields(spec any, datasetSlugs []string, opts Options, chartID string) []string {
	if len(datasetSlugs) == 0 {
		return nil
	}
	slug := datasetSlugs[0]
	columns := datasetColumns(slug, opts)
	if len(columns) == 0 {
		return nil
	}
	columnSet := map[string]bool{}
	for _, col := range columns {
		columnSet[col] = true
	}
	var out []string
	for _, field := range collectFields(spec) {
		if !columnSet[field] {
			out = append(out, fmt.Sprintf("field '%s' not in [[%s]] columns (chart=%s)", field, slug, chartID))
		}
	}
	return out
}

func collectFields(node any) []string {
	var out []string
	var walk func(any)
	walk = func(n any) {
		switch typed := n.(type) {
		case map[string]any:
			if encoding, ok := typed["encoding"].(map[string]any); ok {
				for _, enc := range encoding {
					if encMap, ok := enc.(map[string]any); ok {
						if field, ok := encMap["field"].(string); ok {
							out = append(out, field)
						}
					}
				}
			}
			for _, value := range typed {
				walk(value)
			}
		case []any:
			for _, value := range typed {
				walk(value)
			}
		}
	}
	walk(node)
	return out
}

func datasetColumns(slug string, opts Options) []string {
	path := filepath.Join(opts.ContentDir, "datasets", slug+".md")
	page, err := wiki.ParsePage(path, opts.ContentDir)
	if err != nil {
		return nil
	}
	var columns []string
	inColumns := false
	itemRe := regexp.MustCompile(`^\s*-\s*\{(.+)\}\s*$`)
	for _, line := range page.FrontRawLines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "columns:") {
			inColumns = true
			continue
		}
		if inColumns && trimmed != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			break
		}
		match := itemRe.FindStringSubmatch(line)
		if !inColumns || match == nil {
			continue
		}
		for _, part := range strings.Split(match[1], ",") {
			key, value, ok := strings.Cut(part, ":")
			if ok && strings.TrimSpace(key) == "name" {
				columns = append(columns, strings.Trim(strings.TrimSpace(value), `"'`))
			}
		}
	}
	return columns
}

func datasetIsPrivate(slug string, opts Options) bool {
	for _, rel := range []string{
		filepath.Join("datasets", slug+".md"),
		filepath.Join("datasets", "private", slug+".md"),
	} {
		page, err := wiki.ParsePage(filepath.Join(opts.ContentDir, rel), opts.ContentDir)
		if err != nil {
			continue
		}
		if strings.Contains(page.RelPath, "/private/") || hasPrivateTag(page.Tags) {
			return true
		}
	}
	return false
}

func pageIsPrivate(page wiki.Page) bool {
	return strings.Contains(page.RelPath, "/private/") || strings.HasPrefix(page.RelPath, "private/") || hasPrivateTag(page.Tags)
}

func hasPrivateTag(tags []string) bool {
	for _, tag := range tags {
		if tag == "private" {
			return true
		}
	}
	return false
}

func chartPreviewEdits(page wiki.Page) []string {
	re := regexp.MustCompile(`(?s)<!-- BEGIN chart-preview:([a-z0-9][a-z0-9-]*) -->\n(.*?)\n<!-- END chart-preview:([a-z0-9][a-z0-9-]*) -->`)
	matches := re.FindAllStringSubmatch(page.Body, -1)
	var out []string
	for _, match := range matches {
		chartID := match[1]
		if match[3] != chartID {
			continue
		}
		body := strings.TrimSpace(match[2])
		if body == "" {
			continue
		}
		expected := regexp.MustCompile(`^!\[` + regexp.QuoteMeta(chartID) + `\]\([^)]+\)$`)
		if !expected.MatchString(body) {
			out = append(out, fmt.Sprintf("hand-edit detected inside chart-preview:%s (re-run charts-render)", chartID))
		}
	}
	return out
}

func replaceFrontmatterScalar(path, key, value string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.SplitAfter(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			break
		}
		if strings.HasPrefix(strings.TrimSpace(lines[i]), key+":") {
			lines[i] = leadingWhitespace(lines[i]) + key + ": " + value + "\n"
			return os.WriteFile(path, []byte(strings.Join(lines, "")), 0644)
		}
	}
	return nil
}

func leadingWhitespace(s string) string {
	var out strings.Builder
	for _, r := range s {
		if r != ' ' && r != '\t' {
			break
		}
		out.WriteRune(r)
	}
	return out.String()
}

func diag(level Level, file, code, msg string) Diagnostic {
	return Diagnostic{Level: level, File: file, Code: code, Message: msg}
}

func sortDiagnostics(diagnostics []Diagnostic) {
	sort.SliceStable(diagnostics, func(i, j int) bool {
		if diagnostics[i].File != diagnostics[j].File {
			return diagnostics[i].File < diagnostics[j].File
		}
		if diagnostics[i].Code != diagnostics[j].Code {
			return diagnostics[i].Code < diagnostics[j].Code
		}
		return diagnostics[i].Message < diagnostics[j].Message
	})
}
