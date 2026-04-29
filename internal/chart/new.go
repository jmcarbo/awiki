package chart

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// NewOptions configures the Runner.New verb.
type NewOptions struct {
	Slug     string
	DataSlug string
}

var slugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// New creates a new chart page scaffold, byte-equivalent to cmd_new in
// scripts/chart.sh. Emits "CHART|created <path>" to stdout on success.
func (r *Runner) New(opts NewOptions, stdout io.Writer) error {
	if opts.Slug == "" {
		return fmt.Errorf("CHART|ERROR|missing <slug>")
	}
	if !slugRE.MatchString(opts.Slug) {
		return fmt.Errorf("CHART|ERROR|invalid slug: %s", opts.Slug)
	}
	if opts.DataSlug == "" {
		return fmt.Errorf("CHART|ERROR|--data=<dataset-slug> required")
	}
	if !slugRE.MatchString(opts.DataSlug) {
		return fmt.Errorf("CHART|ERROR|invalid dataset slug: %s", opts.DataSlug)
	}

	chartsDir := filepath.Join(r.ContentDir, "charts")
	datasetsDir := filepath.Join(r.ContentDir, "datasets")
	page := filepath.Join(chartsDir, opts.Slug+".md")

	if _, err := os.Stat(page); err == nil {
		return fmt.Errorf("CHART|ERROR|%s already exists", page)
	}

	datasetPage := filepath.Join(datasetsDir, opts.DataSlug+".md")
	if _, err := os.Stat(datasetPage); err != nil {
		return fmt.Errorf("CHART|ERROR|dataset not found: %s", opts.DataSlug)
	}

	if err := os.MkdirAll(chartsDir, 0755); err != nil {
		return fmt.Errorf("CHART|ERROR|mkdir: %w", err)
	}

	today := time.Now().Format("2006-01-02")
	content := chartScaffold(opts.Slug, opts.DataSlug, today)

	if err := os.WriteFile(page, []byte(content), 0644); err != nil {
		return fmt.Errorf("CHART|ERROR|write: %w", err)
	}

	fmt.Fprintf(stdout, "CHART|created %s\n", page)
	return nil
}

// chartScaffold returns the scaffold content for a new chart page.
// It matches the heredoc in cmd_new byte-for-byte.
func chartScaffold(slug, data, today string) string {
	return `---
title: "` + slug + `"
date: ` + today + `
last_updated: ` + today + `
type: chart
chart_engine: vega-lite
chart_data: "[[` + data + `]]"
sources: ["[[` + data + `]]"]
draft: false
---

# ` + slug + `

<!-- one-paragraph description here -->

` + "```vega-lite" + `
{
  "mark": "bar",
  "data": {"name": "[[` + data + `]]"},
  "encoding": {
    "x": {"field": "<x-field>", "type": "nominal"},
    "y": {"field": "<y-field>", "type": "quantitative"}
  }
}
` + "```" + `

## Notes

## Related
`
}
