package query

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"awiki/internal/dataset"
)

var (
	// reSlug matches valid awiki slug identifiers.
	reSlug = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

	// reLineComment matches -- line comments.
	reLineComment = regexp.MustCompile(`--[^\n]*`)

	// reBlockComment matches /* ... */ block comments.
	reBlockComment = regexp.MustCompile(`(?s)/\*.*?\*/`)

	// reStringLiteral matches SQL string literals (with escaped single-quotes).
	reStringLiteral = regexp.MustCompile(`'(?:''|[^'])*'`)

	// reFromJoin matches FROM or JOIN clause references.
	reFromJoin = regexp.MustCompile(`(?i)\b(?:FROM|JOIN)\s+(?:(?:main|public)\.)?\"?([a-zA-Z_][\w-]*)\"?`)

	// reCTENames matches WITH ... SELECT to find CTE names.
	reCTENames = regexp.MustCompile(`(?is)\bWITH\b(.+?)\bSELECT\b`)

	// reCTEDef matches individual CTE definitions.
	reCTEDef = regexp.MustCompile(`(?i)([a-zA-Z_]\w*)\s+AS\s*\(`)
)

// stripStringsAndComments removes comments and string literals from SQL
// (mirrors the python helper _strip_strings_and_comments).
func stripStringsAndComments(sql string) string {
	sql = reLineComment.ReplaceAllString(sql, "")
	sql = reBlockComment.ReplaceAllString(sql, "")
	sql = reStringLiteral.ReplaceAllString(sql, "''")
	return sql
}

// cteNames extracts CTE names defined with WITH ... AS ( ... ).
func cteNames(sql string) map[string]bool {
	names := map[string]bool{}
	for _, m := range reCTENames.FindAllStringSubmatch(sql, -1) {
		block := m[1]
		for _, cte := range reCTEDef.FindAllStringSubmatch(block, -1) {
			names[strings.ToLower(cte[1])] = true
		}
	}
	return names
}

// refsFromSQL extracts the dataset slug names referenced in FROM/JOIN clauses,
// sorted lexicographically (matching query-resolve.py).
func refsFromSQL(sql string) []string {
	stripped := stripStringsAndComments(sql)
	skip := cteNames(stripped)
	seen := map[string]bool{}
	var found []string
	for _, m := range reFromJoin.FindAllStringSubmatch(stripped, -1) {
		name := strings.ToLower(m[1])
		if skip[name] {
			continue
		}
		if !reSlug.MatchString(name) {
			continue
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		found = append(found, name)
	}
	sort.Strings(found)
	return found
}

// viewDDL returns the CREATE VIEW DDL statement for a slug backed by a file.
func viewDDL(slug, filePath string, fmt dataset.Format) string {
	switch fmt {
	case dataset.FormatJSON, dataset.FormatTopoJSON:
		return fmt2Sprintf("CREATE VIEW %s AS SELECT * FROM read_json_auto('%s');\n", slug, filePath)
	default:
		return fmt2Sprintf("CREATE VIEW %s AS SELECT * FROM read_csv('%s', AUTO_DETECT=TRUE);\n", slug, filePath)
	}
}

func fmt2Sprintf(format, slug, path string) string {
	return strings.Replace(strings.Replace(format, "%s", slug, 1), "%s", path, 1)
}

// ResolveSQL rewrites the SQL so that all referenced dataset slugs are
// accessible as DuckDB views. It returns the full script (CREATE VIEW DDL
// prepended to sql), a list of temp file paths that should be cleaned up
// by the caller, and any error.
//
// For inline-storage datasets the data fence body is written to a temp file.
// For file-storage datasets the data_path is used directly.
func ResolveSQL(repoRoot, sql string) (rewrittenSQL string, tempPaths []string, err error) {
	datasetsDir := filepath.Join(repoRoot, "content", "datasets")
	return resolveSQLWithDir(datasetsDir, repoRoot, sql)
}

// resolveSQLWithDir is the testable version that accepts an explicit datasetsDir.
func resolveSQLWithDir(datasetsDir, repoRoot, sql string) (rewrittenSQL string, tempPaths []string, err error) {
	slugs := refsFromSQL(sql)

	var ddlParts []string
	for _, slug := range slugs {
		pagePath := filepath.Join(datasetsDir, slug+".md")
		pageBytes, readErr := os.ReadFile(pagePath)
		if readErr != nil {
			return "", tempPaths, fmt.Errorf("unknown dataset: %s", slug)
		}
		text := string(pageBytes)

		fmtStr, _ := dataset.FmGet(text, "format")
		storageStr, _ := dataset.FmGet(text, "storage")
		if storageStr == "" {
			storageStr = "inline"
		}
		fmt := dataset.Format(fmtStr)

		var dataFilePath string
		if storageStr == "file" {
			dataPath, ok := dataset.FmGet(text, "data_path")
			if !ok || dataPath == "" {
				return "", tempPaths, fmt2Errorf("storage=file but no data_path for dataset: %s", slug)
			}
			if !filepath.IsAbs(dataPath) {
				dataPath = filepath.Join(repoRoot, dataPath)
			}
			dataFilePath = dataPath
		} else {
			// inline: extract the data fence and write to temp file.
			body, _, ok := dataset.ExtractDataFence(text)
			if !ok {
				return "", tempPaths, fmt2Errorf("no data fence in dataset: %s", slug)
			}
			ext := string(fmt)
			if ext == "" {
				ext = "csv"
			}
			tmp, tmpErr := os.CreateTemp("", "awiki-query-*."+ext)
			if tmpErr != nil {
				return "", tempPaths, tmpErr
			}
			tmpPath := tmp.Name()
			tempPaths = append(tempPaths, tmpPath)
			content := body
			if len(content) > 0 && content[len(content)-1] != '\n' {
				content = append(content, '\n')
			}
			if _, writeErr := tmp.Write(content); writeErr != nil {
				tmp.Close()
				return "", tempPaths, writeErr
			}
			if closeErr := tmp.Close(); closeErr != nil {
				return "", tempPaths, closeErr
			}
			dataFilePath = tmpPath
		}

		ddlParts = append(ddlParts, viewDDL(slug, dataFilePath, fmt))
	}

	ddl := strings.Join(ddlParts, "")
	if ddl != "" {
		rewrittenSQL = ddl + sql + "\n"
	} else {
		rewrittenSQL = sql
	}
	return rewrittenSQL, tempPaths, nil
}

func fmt2Errorf(format, slug string) error {
	return fmt.Errorf("%s", strings.Replace(format, "%s", slug, 1))
}

// CleanupTemps removes temp files created by ResolveSQL.
func CleanupTemps(paths []string) {
	for _, p := range paths {
		os.Remove(p)
	}
}
