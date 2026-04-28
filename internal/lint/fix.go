package lint

import (
	"fmt"
	"os"
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
	for i, line := range lines {
		if !strings.HasPrefix(line, "date: ") {
			continue
		}
		insert := fmt.Sprintf("last_updated: %s\n", today)
		lines = append(lines[:i+1], append([]string{insert}, lines[i+1:]...)...)
		if err := os.WriteFile(path, []byte(strings.Join(lines, "")), 0o644); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}
