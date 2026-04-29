package query

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

// NewOptions configures the New verb.
type NewOptions struct {
	Slug    string
	OutSlug string
}

var reValidSlug = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// New creates a new type:query page scaffold.
func (r *Runner) New(opts NewOptions, stdout io.Writer) error {
	if opts.Slug == "" {
		return fmt.Errorf("QUERY|ERROR|missing <slug>")
	}
	if !reValidSlug.MatchString(opts.Slug) {
		return fmt.Errorf("QUERY|ERROR|invalid slug: %s", opts.Slug)
	}
	if opts.OutSlug == "" {
		return fmt.Errorf("QUERY|ERROR|--out=<dataset-slug> required")
	}
	if !reValidSlug.MatchString(opts.OutSlug) {
		return fmt.Errorf("QUERY|ERROR|invalid out slug: %s", opts.OutSlug)
	}

	page := filepath.Join(r.QueryDir(), opts.Slug+".md")
	if _, err := os.Stat(page); err == nil {
		return fmt.Errorf("QUERY|ERROR|%s already exists", page)
	}

	if err := os.MkdirAll(r.QueryDir(), 0755); err != nil {
		return err
	}

	today := r.Today
	if today == "" {
		today = nowDate()
	}

	content := queryScaffold(opts.Slug, opts.OutSlug, today)
	if err := os.WriteFile(page, []byte(content), 0644); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "QUERY|NEW|%s\n", page)
	return nil
}

// queryScaffold generates the content of a new type:query page matching
// the cmd_new heredoc in scripts/query.sh byte-for-byte.
func queryScaffold(slug, outSlug, today string) string {
	return fmt.Sprintf(`---
title: "%s"
date: %s
last_updated: %s
type: query
sources: []
out: %s
privacy: internal
deterministic: true
sql_hash: ""
draft: false
---

# %s

<!-- describe what this query answers -->

## SQL

`+"```"+`sql
SELECT 1 AS placeholder
ORDER BY 1
`+"```"+`

## Notes
`, slug, today, today, outSlug, slug)
}
