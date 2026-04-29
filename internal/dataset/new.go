package dataset

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

var slugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

var supportedFormats = map[Format]bool{
	FormatCSV:      true,
	FormatTSV:      true,
	FormatJSON:     true,
	FormatDSV:      true,
	FormatTopoJSON: true,
}

// NewOptions holds parameters for creating a new dataset page.
type NewOptions struct {
	Slug   string
	Format string
	From   string // optional source data file path
}

// New creates a new dataset page under DatasetDir. It refuses to overwrite an
// existing page and validates slug + format. If From is set, reads the source
// file and embeds it inline. Emits DATASET|created <page> (rows=<N>) to stdout.
func (r *Runner) New(opts NewOptions, stdout io.Writer) error {
	if opts.Slug == "" {
		return fmt.Errorf("missing <slug>")
	}
	if !slugRE.MatchString(opts.Slug) {
		return fmt.Errorf("invalid slug: %s (must match ^[a-z0-9][a-z0-9-]*$)", opts.Slug)
	}

	format := Format(opts.Format)
	if !supportedFormats[format] {
		return fmt.Errorf("unsupported format: %s", opts.Format)
	}

	if err := os.MkdirAll(r.DatasetDir(), 0o755); err != nil {
		return err
	}

	page := filepath.Join(r.DatasetDir(), opts.Slug+".md")
	if _, err := os.Stat(page); err == nil {
		return fmt.Errorf("%s already exists", page)
	}

	today := r.Today
	if today == "" {
		today = time.Now().Format("2006-01-02")
	}

	var body []byte
	rows := 0
	if opts.From != "" {
		src, err := os.ReadFile(opts.From)
		if err != nil {
			return fmt.Errorf("source file not found: %s", opts.From)
		}
		body = src
		// Count data rows (header excluded).
		parsed, err := LoadInlineRows(format, src)
		if err != nil {
			return fmt.Errorf("cannot read source file: %v", err)
		}
		rows = len(parsed)
		if rows > 0 {
			rows-- // strip header
		}
	}

	scaffold := WriteScaffold(ScaffoldInput{
		Slug:   opts.Slug,
		Format: format,
		Today:  today,
		Rows:   rows,
		Body:   body,
	})

	if err := os.WriteFile(page, scaffold, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "DATASET|created %s (rows=%d)\n", page, rows)
	return nil
}
