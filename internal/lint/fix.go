package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"awiki/internal/wiki"
)

func applyLastUpdatedFixes(c *Collector, opts Options, pages []wiki.Page) {
	today := opts.Today
	if today == "" {
		today = time.Now().Format("2006-01-02")
	}

	for _, page := range pages {
		if page.Date == "" || page.LastUpdated != "" {
			continue
		}
		fixed, err := addLastUpdatedAfterDate(page.Path, today)
		if err != nil {
			c.Add(Diagnostic{
				Level:   Error,
				File:    page.Path,
				Message: err.Error(),
			})
			continue
		}
		if !fixed {
			continue
		}
		c.AddFix(FixRecord{
			File:    page.Path,
			Message: fmt.Sprintf("added last_updated: %s", today),
		})
	}
}

func addLastUpdatedAfterDate(path, today string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	lines := strings.SplitAfter(string(data), "\n")
	frontmatterEnd := closingFrontmatterLine(lines)
	if frontmatterEnd == -1 {
		return false, nil
	}

	for i, line := range lines[1:frontmatterEnd] {
		if !strings.HasPrefix(line, "date: ") {
			continue
		}
		insertAt := i + 2
		insert := fmt.Sprintf("last_updated: %s\n", today)
		lines = append(lines[:insertAt], append([]string{insert}, lines[insertAt:]...)...)
		if err := atomicWriteFile(path, []byte(strings.Join(lines, ""))); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func atomicWriteFile(path string, data []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	keepTemp := true
	defer func() {
		if keepTemp {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		return closeTempFile(tmp, err)
	}
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		return closeTempFile(tmp, err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	keepTemp = false
	return nil
}

func closeTempFile(file *os.File, err error) error {
	if closeErr := file.Close(); closeErr != nil && err == nil {
		return closeErr
	}
	return err
}

func closingFrontmatterLine(lines []string) int {
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return -1
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return i
		}
	}
	return -1
}
