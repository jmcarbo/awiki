package query

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"awiki/internal/region"
)

// FenceRender walks ContentDir for markdown files with awiki-query fences,
// executes each fence's SQL, and replaces the fence body with the rendered
// table via managed regions. Also writes a .queries.json sidecar per page.
func (r *Runner) FenceRender(ctx context.Context, stdout io.Writer) error {
	// Collect all pages with awiki-query fences.
	type fenceEntry struct {
		page  string
		fid   string // id= attribute from info-line, or short hash
		sql   string
	}

	var entries []fenceEntry
	err := walkMarkdown(r.ContentDir, func(path string) error {
		textBytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(textBytes)
		fences := ExtractFences(text)
		for _, f := range fences {
			// Derive a fence ID: use Out slug if present, else short hash of SQL.
			fid := f.Out
			if fid == "" {
				h := CanonicalHash(Result{Columns: []string{f.SQL}})
				if len(h) > 8 {
					h = h[:8]
				}
				fid = h
			}
			entries = append(entries, fenceEntry{page: path, fid: fid, sql: f.SQL})
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Per-page map of fid -> hash for sidecar.
	pageHashes := map[string]map[string]string{}
	var firstErr error

	for _, e := range entries {
		rewritten, tmps, resolveErr := ResolveSQL(r.RepoRoot, e.sql)
		if resolveErr != nil {
			if firstErr == nil {
				firstErr = resolveErr
			}
			continue
		}
		out, code, execErr := r.DuckDB.Run(ctx, rewritten, "json")
		CleanupTemps(tmps)
		if execErr != nil || code != 0 {
			if firstErr == nil {
				if execErr != nil {
					firstErr = execErr
				} else {
					firstErr = fmt.Errorf("QUERY|ERROR|duckdb exit %d for fence %s in %s", code, e.fid, e.page)
				}
			}
			continue
		}

		result, parseErr := parseJSONRows(out)
		if parseErr != nil {
			if firstErr == nil {
				firstErr = parseErr
			}
			continue
		}

		body := FormatTable(result)

		// Read the page, apply managed region replacement, write back.
		pageBytes, readErr := os.ReadFile(e.page)
		if readErr != nil {
			if firstErr == nil {
				firstErr = readErr
			}
			continue
		}
		newText, replaceErr := region.ManagedReplace(string(pageBytes), "awiki-query", e.fid, body)
		if replaceErr != nil {
			if firstErr == nil {
				firstErr = replaceErr
			}
			continue
		}
		writeErr := os.WriteFile(e.page, []byte(newText), 0644)
		if writeErr != nil {
			if firstErr == nil {
				firstErr = writeErr
			}
			continue
		}

		// Compute hash.
		sqlHash := sqlHashOf(e.sql)

		if pageHashes[e.page] == nil {
			pageHashes[e.page] = map[string]string{}
		}
		pageHashes[e.page][e.fid] = "sha256-" + sqlHash

		fmt.Fprintf(stdout, "QUERY|FENCE-RENDER|%s|id=%s|hash=sha256-%s\n", e.page, e.fid, sqlHash)
	}

	// Write .queries.json sidecars.
	for page, hashes := range pageHashes {
		sidecar := strings.TrimSuffix(page, filepath.Ext(page)) + ".queries.json"
		data, marshalErr := json.MarshalIndent(hashes, "", "  ")
		if marshalErr == nil {
			os.WriteFile(sidecar, append(data, '\n'), 0644)
		}
	}

	return firstErr
}

// walkMarkdown calls fn for every .md file under root.
func walkMarkdown(root string, fn func(path string) error) error {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		fullPath := filepath.Join(root, e.Name())
		if e.IsDir() {
			if walkErr := walkMarkdown(fullPath, fn); walkErr != nil {
				return walkErr
			}
			continue
		}
		if strings.HasSuffix(e.Name(), ".md") {
			if fnErr := fn(fullPath); fnErr != nil {
				return fnErr
			}
		}
	}
	return nil
}
