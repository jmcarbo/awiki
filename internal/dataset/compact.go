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

// Compact adjusts the storage mode of a dataset between inline and file
// storage based on the configured inline thresholds. It emits a DATASET|...
// record to stdout describing the action taken.
func (r *Runner) Compact(slug string, stdout io.Writer) error {
	page := filepath.Join(r.DatasetDir(), slug+".md")
	data, err := os.ReadFile(page)
	if err != nil {
		return fmt.Errorf("%s not found", page)
	}
	text := string(data)

	storage, _ := FmGet(text, "storage")
	format, _ := FmGet(text, "format")

	var rawBytes []byte
	if Storage(storage) == StorageInline {
		body, _, ok := ExtractDataFence(text)
		if !ok {
			return fmt.Errorf("missing inline data fence in %s", page)
		}
		rawBytes = body
	} else {
		dp, _ := FmGet(text, "data_path")
		rawBytes, err = os.ReadFile(filepath.Join(r.RepoRoot, dp))
		if err != nil {
			return err
		}
	}

	rows, err := LoadInlineRows(Format(format), rawBytes)
	if err != nil {
		return err
	}
	rowsCount := len(rows)
	if rowsCount > 0 {
		rowsCount-- // header excluded (matches python DictReader convention)
	}
	size := len(rawBytes)
	maxRows, maxBytes := InlineThresholds(r.RepoRoot)
	over := rowsCount > maxRows || size > maxBytes

	if Storage(storage) == StorageInline && over {
		// Move inline -> file.
		if err := os.MkdirAll(r.DataDir, 0o755); err != nil {
			return err
		}
		target := filepath.Join(r.DataDir, slug+"."+format)
		if err := os.WriteFile(target, rawBytes, 0o644); err != nil {
			return err
		}
		text = RemoveDataFence(text)
		text = FmSet(text, "storage", "file")
		// data_path is relative to repo root, matching bash convention
		relTarget, err := filepath.Rel(r.RepoRoot, target)
		if err != nil {
			relTarget = filepath.Join("data", slug+"."+format)
		}
		text = FmSet(text, "data_path", relTarget)
		text = FmSet(text, "rows", strconv.Itoa(rowsCount))
		if err := fsutil.AtomicWrite(page, []byte(text)); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "DATASET|compacted %s inline -> file (%s, rows=%d, bytes=%d)\n",
			slug, target, rowsCount, size)
		return nil
	}

	if Storage(storage) == StorageFile && !over {
		if !strings.Contains(text, "## Provenance") {
			return fmt.Errorf("%s missing '## Provenance' heading; cannot insert ## Data block", page)
		}
		var dp string
		dp, _ = FmGet(text, "data_path")
		newText, err := ReplaceDataFence(text, Format(format), rawBytes)
		if err != nil {
			return err
		}
		newText = FmRemove(newText, "data_path")
		newText = FmSet(newText, "storage", "inline")
		newText = FmSet(newText, "rows", strconv.Itoa(rowsCount))
		if err := fsutil.AtomicWrite(page, []byte(newText)); err != nil {
			return err
		}
		// Remove the data file we just inlined.
		if dp != "" {
			_ = os.Remove(filepath.Join(r.RepoRoot, dp))
		}
		fmt.Fprintf(stdout, "DATASET|compacted %s file -> inline (rows=%d, bytes=%d)\n",
			slug, rowsCount, size)
		return nil
	}

	if Storage(storage) == StorageFile && over {
		return fmt.Errorf("%s is over threshold (rows=%d, bytes=%d); cannot inline", slug, rowsCount, size)
	}

	// Already in the right mode.
	fmt.Fprintf(stdout, "DATASET|%s already compact (storage=%s, rows=%d, bytes=%d)\n",
		slug, storage, rowsCount, size)
	return nil
}
