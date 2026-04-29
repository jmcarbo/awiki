package chart

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"awiki/internal/wiki"
)

func renderOneImpl(r *Runner, chartID string, stdout, stderr io.Writer) error {
	assetsDir := r.AssetsChartsDir()
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return fmt.Errorf("mkdir assets: %w", err)
	}

	found := false
	var lastErr error

	err := filepath.WalkDir(r.ContentDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}
		textBytes, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		pageText := string(textBytes)
		page, parseErr := wiki.ParsePage(path, r.ContentDir)
		if parseErr != nil {
			return nil
		}
		slug := page.Slug
		isDedicated := page.Type == "chart"
		fences := ExtractFences(pageText, slug, isDedicated)
		for _, fence := range fences {
			if fence.ID != chartID {
				continue
			}
			found = true
			_, err := renderFence(r, path, pageText, fence, assetsDir, stdout, stderr)
			if err != nil {
				lastErr = err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if lastErr != nil {
		return lastErr
	}
	if !found {
		return fmt.Errorf("CHART|ERROR|unknown chart: %s", chartID)
	}
	return nil
}
