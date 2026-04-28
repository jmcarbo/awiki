package data

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"awiki/internal/wiki"
)

func Run(opts Options) ([]Diagnostic, []FixRecord, error) {
	pages, err := datasetPages(opts)
	if err != nil {
		return nil, nil, err
	}
	var diagnostics []Diagnostic
	var fixes []FixRecord
	for _, page := range pages {
		ds, err := parseDataset(page, opts)
		if err != nil {
			return nil, nil, err
		}
		pageDiagnostics, pageFixes := lintDataset(ds, opts)
		diagnostics = append(diagnostics, pageDiagnostics...)
		fixes = append(fixes, pageFixes...)
	}
	return diagnostics, fixes, nil
}

func datasetPages(opts Options) ([]wiki.Page, error) {
	var paths []string
	if opts.OnlyFile != "" {
		paths = append(paths, resolveOnlyFile(opts))
	} else {
		root := filepath.Join(opts.ContentDir, "datasets")
		if _, err := os.Stat(root); os.IsNotExist(err) {
			return nil, nil
		}
		if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
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
	}
	pages := make([]wiki.Page, 0, len(paths))
	for _, path := range paths {
		page, err := wiki.ParsePage(path, opts.ContentDir)
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(page.RelPath, "datasets/") || page.Type == "dataset" {
			pages = append(pages, page)
		}
	}
	return pages, nil
}

func resolveOnlyFile(opts Options) string {
	if filepath.IsAbs(opts.OnlyFile) {
		return opts.OnlyFile
	}
	if strings.HasPrefix(filepath.ToSlash(opts.OnlyFile), "content/") {
		return filepath.Join(opts.RepoRoot, filepath.FromSlash(opts.OnlyFile))
	}
	return filepath.Join(opts.ContentDir, filepath.FromSlash(opts.OnlyFile))
}

type dataset struct {
	page       wiki.Page
	storage    string
	format     string
	dataPath   string
	rows       int
	hasRows    bool
	sourcePath string
	sourceData string
	sourceSize int
	columns    []column
}

func parseDataset(page wiki.Page, opts Options) (dataset, error) {
	ds := dataset{
		page:     page,
		storage:  page.Frontmatter["storage"],
		format:   page.Frontmatter["format"],
		dataPath: page.Frontmatter["data_path"],
		columns:  parseColumns(page.FrontRawLines),
	}
	if raw := page.Frontmatter["rows"]; raw != "" {
		if rows, err := strconv.Atoi(raw); err == nil {
			ds.rows = rows
			ds.hasRows = true
		}
	}
	if ds.storage == "file" && ds.dataPath != "" {
		if filepath.IsAbs(ds.dataPath) {
			ds.sourcePath = ds.dataPath
		} else {
			ds.sourcePath = filepath.Join(opts.RepoRoot, filepath.FromSlash(ds.dataPath))
		}
	}
	if ds.storage == "inline" && ds.format != "" {
		info, body, ok := inlineDataFence(page.Body)
		if ok {
			ds.sourceData = body
			ds.sourceSize = len([]byte(body))
			if info != "" && info != ds.format {
				ds.sourcePath = "format-mismatch:" + info
			}
		}
	}
	return ds, nil
}

func lintDataset(ds dataset, opts Options) ([]Diagnostic, []FixRecord) {
	var diagnostics []Diagnostic
	var fixes []FixRecord
	file := "content/" + ds.page.RelPath
	if ds.storage == "" {
		return []Diagnostic{diag(Error, file, "D1", "missing storage")}, nil
	}
	if ds.format == "" {
		return []Diagnostic{diag(Error, file, "D1", "missing format")}, nil
	}

	var dataFile string
	var cleanup func()
	if ds.storage == "file" {
		if ds.dataPath == "" {
			return []Diagnostic{diag(Error, file, "D2", "storage=file but data_path missing")}, nil
		}
		if _, err := os.Stat(ds.sourcePath); err != nil {
			return []Diagnostic{diag(Error, file, "D2", "data file absent: "+ds.dataPath)}, nil
		}
		dataFile = ds.sourcePath
	}
	if ds.storage == "inline" {
		info, body, ok := inlineDataFence(ds.page.Body)
		if !ok {
			return []Diagnostic{diag(Error, file, "D3", "storage=inline but no fenced block under ## Data")}, nil
		}
		if info != ds.format {
			diagnostics = append(diagnostics, diag(Error, file, "D4", fmt.Sprintf("frontmatter format=%s != fence info=%s", ds.format, info)))
		}
		tmp, err := os.CreateTemp("", "awiki-dataset-*."+ds.format)
		if err != nil {
			diagnostics = append(diagnostics, diag(Error, file, "D5", err.Error()))
		} else {
			_, _ = tmp.WriteString(body)
			_ = tmp.Close()
			dataFile = tmp.Name()
			cleanup = func() { _ = os.Remove(tmp.Name()) }
		}
	}
	if cleanup != nil {
		defer cleanup()
	}

	if dataFile != "" {
		rows, err := loadRows(dataFile, ds.format)
		if err != nil {
			diagnostics = append(diagnostics, diag(Error, file, "D5", err.Error()))
		} else {
			if len(ds.columns) > 0 {
				for _, msg := range validateRows(sampleRows(rows.rows), ds.columns) {
					diagnostics = append(diagnostics, diag(Error, file, "D5", msg))
				}
			}
			if ds.storage == "inline" {
				maxRows, maxBytes := inlineThresholds(opts.RepoRoot)
				if len(rows.rows) > maxRows || ds.sourceSize > maxBytes {
					diagnostics = append(diagnostics, diag(Warn, file, "D6", fmt.Sprintf("inline dataset over threshold (rows=%d bytes=%d); run dataset-compact", len(rows.rows), ds.sourceSize)))
				}
			}
			if !ds.hasRows || ds.rows != len(rows.rows) {
				if opts.Fix {
					if replaceFrontmatterScalar(ds.page.Path, "rows", strconv.Itoa(len(rows.rows))) == nil {
						fixes = append(fixes, FixRecord{File: file, Message: fmt.Sprintf("rows: %d -> %d", ds.rows, len(rows.rows))})
					}
				} else {
					diagnostics = append(diagnostics, diag(Warn, file, "D7", fmt.Sprintf("declared rows=%d but actual=%d (run with --fix)", ds.rows, len(rows.rows))))
				}
			}
		}
	}

	if ds.storage == "file" && ds.dataPath != "" && !strings.HasPrefix(ds.dataPath, "data/") {
		diagnostics = append(diagnostics, diag(Warn, file, "D8", "data_path outside data/: "+ds.dataPath))
	}
	if frontmatterHasEmptySources(ds.page.FrontRawLines) {
		diagnostics = append(diagnostics, diag(Info, file, "D9", "no sources declared"))
	}
	return diagnostics, fixes
}

func diag(level Level, file, code, msg string) Diagnostic {
	return Diagnostic{Level: level, File: file, Code: code, Message: msg}
}

func inlineDataFence(body string) (string, string, bool) {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	inData := false
	for i, line := range lines {
		if strings.TrimSpace(line) == "## Data" {
			inData = true
			continue
		}
		if !inData {
			continue
		}
		if strings.HasPrefix(line, "```") {
			info := strings.TrimSpace(strings.TrimPrefix(line, "```"))
			var bodyLines []string
			for _, dataLine := range lines[i+1:] {
				if strings.TrimSpace(dataLine) == "```" {
					return info, strings.Join(bodyLines, "\n") + "\n", true
				}
				bodyLines = append(bodyLines, dataLine)
			}
			return info, strings.Join(bodyLines, "\n"), true
		}
	}
	return "", "", false
}

func parseColumns(lines []string) []column {
	var columns []column
	inColumns := false
	itemRe := regexp.MustCompile(`^\s*-\s*\{(.+)\}\s*$`)
	for _, line := range lines {
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
		fields := map[string]string{}
		for _, part := range strings.Split(match[1], ",") {
			key, value, ok := strings.Cut(part, ":")
			if ok {
				fields[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"'`)
			}
		}
		if fields["name"] != "" && fields["type"] != "" {
			columns = append(columns, column{Name: fields["name"], Type: fields["type"]})
		}
	}
	return columns
}

func inlineThresholds(repoRoot string) (int, int) {
	maxRows, maxBytes := 500, 51200
	data, err := os.ReadFile(filepath.Join(repoRoot, ".awiki", "config"))
	if err != nil {
		return maxRows, maxBytes
	}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			continue
		}
		switch key {
		case "AWIKI_DATASET_INLINE_MAX_ROWS":
			maxRows = parsed
		case "AWIKI_DATASET_INLINE_MAX_BYTES":
			maxBytes = parsed
		}
	}
	return maxRows, maxBytes
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
	inFront := true
	found := false
	for i := 1; i < len(lines); i++ {
		if inFront && strings.TrimSpace(lines[i]) == "---" {
			if !found {
				lines = append(lines[:i], append([]string{key + ": " + value + "\n"}, lines[i:]...)...)
			}
			break
		}
		if !inFront {
			break
		}
		if strings.HasPrefix(strings.TrimSpace(lines[i]), key+":") {
			prefix := leadingWhitespace(lines[i])
			lines[i] = prefix + key + ": " + value + "\n"
			found = true
			break
		}
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "")), 0644)
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

func frontmatterHasEmptySources(lines []string) bool {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "sources: []" {
			return true
		}
		if strings.HasPrefix(trimmed, "sources:") {
			return false
		}
	}
	return true
}
