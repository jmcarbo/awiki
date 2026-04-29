package dataset

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"awiki/internal/fsutil"
)

// Validate reads the dataset page for slug, validates its data against the
// declared column schema (if any), refreshes the rows: frontmatter key, and
// emits DATASET|validated <slug> (rows=<N>) to stdout.
func (r *Runner) Validate(slug string, stdout, stderr io.Writer) error {
	page := filepath.Join(r.DatasetDir(), slug+".md")
	data, err := os.ReadFile(page)
	if err != nil {
		return fmt.Errorf("%s not found", page)
	}
	text := string(data)

	storage, ok := FmGet(text, "storage")
	if !ok || storage == "" {
		return fmt.Errorf("missing storage in %s", page)
	}
	format, ok := FmGet(text, "format")
	if !ok || format == "" {
		return fmt.Errorf("missing format in %s", page)
	}

	var rows [][]string
	switch Storage(storage) {
	case StorageFile:
		dp, _ := FmGet(text, "data_path")
		if dp == "" {
			return fmt.Errorf("missing data_path for storage=file in %s", page)
		}
		rows, err = LoadRows(Format(format), filepath.Join(r.RepoRoot, dp))
		if err != nil {
			return fmt.Errorf("data_path not found or unreadable: %s", dp)
		}
	case StorageInline:
		body, fenceFmt, ok := ExtractDataFence(text)
		if !ok {
			return fmt.Errorf("missing inline data fence in %s", page)
		}
		if string(fenceFmt) != format {
			return fmt.Errorf("fence format %s does not match frontmatter format %s",
				fenceFmt, format)
		}
		rows, err = LoadInlineRows(Format(format), body)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported storage: %s", storage)
	}

	columns := parseColumnsFromFrontmatter(text)
	if len(columns) > 0 {
		if errs := ValidateRows(rows, columns); len(errs) > 0 {
			for _, e := range errs {
				if e.Missing {
					fmt.Fprintf(stderr, "DATASET|ERROR|row=%d col=%s missing\n", e.Row, e.Column)
				} else {
					fmt.Fprintf(stderr, "DATASET|ERROR|row=%d col=%s want=%s got=%s\n",
						e.Row, e.Column, e.Want, e.Got)
				}
			}
			return fmt.Errorf("schema validation failed for %s", slug)
		}
	}

	actual := len(rows)
	if actual > 0 {
		actual-- // strip header row (matches python DictReader convention)
	}
	updated := FmSet(text, "rows", strconv.Itoa(actual))
	if updated != text {
		if err := fsutil.AtomicWrite(page, []byte(updated)); err != nil {
			return err
		}
	}
	fmt.Fprintf(stdout, "DATASET|validated %s (rows=%d)\n", slug, actual)
	return nil
}

// parseColumnsFromFrontmatter reads inline-list `columns: [{name: x, type: integer}, ...]`
// from frontmatter. Returns empty when absent or empty.
func parseColumnsFromFrontmatter(text string) []Column {
	raw, ok := FmGet(text, "columns")
	if !ok || strings.TrimSpace(raw) == "[]" || strings.TrimSpace(raw) == "" {
		return nil
	}
	// crude inline parser: split outermost `{...}` items
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "[")
	raw = strings.TrimSuffix(raw, "]")
	var out []Column
	depth := 0
	start := 0
	for i, c := range raw {
		switch c {
		case '{':
			if depth == 0 {
				start = i + 1
			}
			depth++
		case '}':
			depth--
			if depth == 0 {
				seg := raw[start:i]
				col := Column{}
				for _, kv := range strings.Split(seg, ",") {
					if eq := strings.Index(kv, ":"); eq > 0 {
						k := strings.TrimSpace(kv[:eq])
						v := strings.TrimSpace(kv[eq+1:])
						v = strings.Trim(v, `"' `)
						switch k {
						case "name":
							col.Name = v
						case "type":
							col.Type = v
						}
					}
				}
				out = append(out, col)
			}
		}
	}
	return out
}
