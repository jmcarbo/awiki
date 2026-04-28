package query

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"awiki/internal/wiki"
)

func Run(opts Options) ([]Diagnostic, error) {
	pages, err := pages(opts)
	if err != nil {
		return nil, err
	}
	var diagnostics []Diagnostic
	for _, page := range pages {
		if strings.HasPrefix(page.RelPath, "queries/") {
			diagnostics = append(diagnostics, lintQueryPage(page, opts)...)
		}
		diagnostics = append(diagnostics, lintInlineFences(page, opts)...)
	}
	sort.SliceStable(diagnostics, func(i, j int) bool {
		if diagnostics[i].File != diagnostics[j].File {
			return diagnostics[i].File < diagnostics[j].File
		}
		if diagnostics[i].Code != diagnostics[j].Code {
			return diagnostics[i].Code < diagnostics[j].Code
		}
		return diagnostics[i].Message < diagnostics[j].Message
	})
	return diagnostics, nil
}

func pages(opts Options) ([]wiki.Page, error) {
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
	out := make([]wiki.Page, 0, len(paths))
	for _, path := range paths {
		page, err := wiki.ParsePage(path, opts.ContentDir)
		if err != nil {
			return nil, err
		}
		out = append(out, page)
	}
	return out, nil
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

func lintQueryPage(page wiki.Page, opts Options) []Diagnostic {
	file := "content/" + page.RelPath
	sql, ok := sqlFence(page.Body)
	if !ok || strings.TrimSpace(sql) == "" {
		return []Diagnostic{diag(Error, file, "Q1", "no ```sql fence found")}
	}
	var out []Diagnostic
	for _, slug := range refs(sql) {
		if _, err := os.Stat(filepath.Join(opts.ContentDir, "datasets", slug+".md")); err != nil {
			out = append(out, diag(Error, file, "Q2", "unknown dataset: "+slug))
		}
	}
	if msg := deterministicError(sql); msg != "" {
		out = append(out, diag(Error, file, "Q3", msg))
	}
	slug := page.Slug
	sidecar := filepath.Join(opts.ContentDir, "queries", slug+".sql.hash")
	if data, err := os.ReadFile(sidecar); err == nil {
		now, err := hashSQL(sql, opts)
		if err == nil && strings.TrimSpace(string(data)) != "sha256-"+now {
			out = append(out, diag(Error, file, "Q4", fmt.Sprintf("sidecar stale: content/queries/%s.sql.hash (run `just query-render-one %s`)", slug, slug)))
		}
	}
	return out
}

func lintInlineFences(page wiki.Page, opts Options) []Diagnostic {
	fences := awikiQueryFences(page.Body)
	if len(fences) == 0 {
		return nil
	}
	sidecarPath := strings.TrimSuffix(page.Path, filepath.Ext(page.Path)) + ".queries.json"
	entries := map[string]string{}
	if data, err := os.ReadFile(sidecarPath); err == nil {
		_ = json.Unmarshal(data, &entries)
	}
	file := "content/" + page.RelPath
	var out []Diagnostic
	for _, fence := range fences {
		now, err := hashSQL(fence.SQL, opts)
		if err != nil || entries[fence.ID] != "sha256-"+now {
			out = append(out, diag(Error, file, "Q5", "managed region stale or tampered (id="+fence.ID+")"))
		}
	}
	return out
}

func sqlFence(body string) (string, bool) {
	re := regexp.MustCompile("(?s)```sql\\s*\\n(.*?)\\n```")
	match := re.FindStringSubmatch(body)
	if match == nil {
		return "", false
	}
	return match[1], true
}

type inlineFence struct {
	ID  string
	SQL string
}

func awikiQueryFences(body string) []inlineFence {
	re := regexp.MustCompile(`(?s)` + "```sql\\s+awiki-query\\s+id=\"([a-z0-9][a-z0-9-]*)\"\\s*\\n(.*?)\\n```")
	matches := re.FindAllStringSubmatch(body, -1)
	out := make([]inlineFence, 0, len(matches))
	for _, match := range matches {
		out = append(out, inlineFence{ID: match[1], SQL: match[2]})
	}
	return out
}

func refs(sql string) []string {
	clean := stripStringsAndComments(sql)
	skip := cteNames(clean)
	re := regexp.MustCompile(`(?i)\b(?:FROM|JOIN)\s+(?:(?:main|public)\.)?"?([a-zA-Z_][\w-]*)"?`)
	seen := map[string]bool{}
	var out []string
	for _, match := range re.FindAllStringSubmatch(clean, -1) {
		name := strings.ToLower(match[1])
		if skip[name] || !regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`).MatchString(name) || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func stripStringsAndComments(sql string) string {
	lineComment := regexp.MustCompile(`--[^\n]*`)
	blockComment := regexp.MustCompile(`(?s)/\*.*?\*/`)
	stringLiteral := regexp.MustCompile(`'(?:''|[^'])*'`)
	sql = lineComment.ReplaceAllString(sql, "")
	sql = blockComment.ReplaceAllString(sql, "")
	sql = stringLiteral.ReplaceAllString(sql, "''")
	return sql
}

func cteNames(sql string) map[string]bool {
	out := map[string]bool{}
	re := regexp.MustCompile(`(?is)\bWITH\b(.+?)\bSELECT\b`)
	item := regexp.MustCompile(`(?i)([a-zA-Z_][\w]*)\s+AS\s*\(`)
	for _, match := range re.FindAllStringSubmatch(sql, -1) {
		for _, cte := range item.FindAllStringSubmatch(match[1], -1) {
			out[strings.ToLower(cte[1])] = true
		}
	}
	return out
}

func deterministicError(sql string) string {
	clean := stripStringsAndComments(sql)
	for _, pattern := range []string{
		`\bNOW\s*\(`,
		`\bCURRENT_TIMESTAMP\b`,
		`\bCURRENT_DATE\b`,
		`\bCURRENT_TIME\b`,
		`\bRANDOM\s*\(`,
		`\bUUID\s*\(`,
		`\bGEN_RANDOM_UUID\s*\(`,
	} {
		re := regexp.MustCompile(`(?i)` + pattern)
		if match := re.FindString(clean); match != "" {
			return "QUERY|ERROR|non-deterministic token: " + strings.TrimSpace(match)
		}
	}
	if !regexp.MustCompile(`(?i)\bORDER\s+BY\b`).MatchString(topLevelSQL(clean)) {
		return "QUERY|ERROR|materialized SQL must have ORDER BY on the outer SELECT"
	}
	return ""
}

func topLevelSQL(sql string) string {
	var out strings.Builder
	depth := 0
	for _, r := range sql {
		switch r {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 {
				out.WriteRune(r)
			}
		}
	}
	return out.String()
}

func hashSQL(sql string, opts Options) (string, error) {
	var payload strings.Builder
	for _, slug := range refs(sql) {
		body, err := datasetBody(slug, opts)
		if err != nil {
			return "", err
		}
		sum := sha256.Sum256([]byte(body))
		payload.WriteString(slug)
		payload.WriteString(":")
		payload.WriteString(hex.EncodeToString(sum[:]))
		payload.WriteString("\n")
	}
	normalized := strings.ToLower(strings.Join(strings.Fields(sql), " "))
	full := normalized + "\n" + payload.String()
	sum := sha256.Sum256([]byte(full))
	return hex.EncodeToString(sum[:]), nil
}

func datasetBody(slug string, opts Options) (string, error) {
	page, err := wiki.ParsePage(filepath.Join(opts.ContentDir, "datasets", slug+".md"), opts.ContentDir)
	if err != nil {
		return "", err
	}
	var body string
	if page.Frontmatter["storage"] == "file" {
		dataPath := page.Frontmatter["data_path"]
		if dataPath == "" {
			return "", fmt.Errorf("storage=file but no data_path: %s", slug)
		}
		path := dataPath
		if !filepath.IsAbs(path) {
			path = filepath.Join(opts.RepoRoot, filepath.FromSlash(path))
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		body = string(data)
	} else {
		format := page.Frontmatter["format"]
		re := regexp.MustCompile("(?s)```" + regexp.QuoteMeta(format) + "\\s*\\n(.*?)\\n```")
		match := re.FindStringSubmatch(page.Body)
		if match == nil {
			return "", fmt.Errorf("no dataset fence: %s", slug)
		}
		body = match[1]
	}
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return body, nil
}

func diag(level Level, file, code, msg string) Diagnostic {
	return Diagnostic{Level: level, File: file, Code: code, Message: msg}
}
