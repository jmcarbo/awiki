package chart

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"awiki/internal/wiki"
)

func renderImpl(r *Runner, opts RenderOptions, stdout, stderr io.Writer) error {
	assetsDir := r.AssetsChartsDir()
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return fmt.Errorf("mkdir assets: %w", err)
	}

	var knownIDs = map[string]struct{}{}
	var lastErr error

	// Walk all .md files in the content directory.
	err := filepath.WalkDir(r.ContentDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			// Skip hidden directories.
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}
		return renderPage(r, path, assetsDir, knownIDs, stdout, stderr, &lastErr)
	})
	if err != nil {
		return err
	}

	if !opts.KeepOrphans {
		if err := cleanOrphans(r, assetsDir, knownIDs, stdout); err != nil && lastErr == nil {
			lastErr = err
		}
	}

	return lastErr
}

func renderPage(r *Runner, pagePath, assetsDir string, knownIDs map[string]struct{}, stdout, stderr io.Writer, lastErr *error) error {
	textBytes, err := os.ReadFile(pagePath)
	if err != nil {
		return err
	}
	pageText := string(textBytes)

	page, parseErr := wiki.ParsePage(pagePath, r.ContentDir)
	if parseErr != nil {
		return nil // skip unparseable pages
	}
	slug := page.Slug
	isDedicated := page.Type == "chart"
	fences := ExtractFences(pageText, slug, isDedicated)

	for _, fence := range fences {
		knownIDs[fence.ID] = struct{}{}
		changed, err := renderFence(r, pagePath, pageText, fence, assetsDir, stdout, stderr)
		if err != nil {
			*lastErr = err
			continue
		}
		if changed {
			// Re-read the page text after injection.
			if b, readErr := os.ReadFile(pagePath); readErr == nil {
				pageText = string(b)
			}
		}
	}
	return nil
}

// renderFence renders a single fence. Returns (pageChanged, error).
func renderFence(r *Runner, pagePath, pageText string, fence ChartFence, assetsDir string, stdout, stderr io.Writer) (bool, error) {
	var spec map[string]any
	if err := json.Unmarshal(fence.Spec, &spec); err != nil {
		fmt.Fprintf(stderr, "CHART|ERROR|%s: JSON parse error: %v\n", fence.ID, err)
		return false, nil
	}

	resolved, err := ResolveSpec(r.RepoRoot, "/", pagePath, spec)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return false, nil
	}

	newHash := Hash(resolved)
	existingHash, hashExists := ReadHash(assetsDir, fence.ID)
	svgPath := filepath.Join(assetsDir, fence.ID+".svg")
	_, svgErr := os.Stat(svgPath)
	if hashExists && newHash == existingHash && svgErr == nil {
		fmt.Fprintf(stdout, "CHART|skip %s (hash match)\n", fence.ID)
		return false, nil
	}

	// Write resolved JSON to a temp file for vl-convert.
	jsonBytes, _ := json.MarshalIndent(resolved, "", "  ")
	tmpJSON, err := os.CreateTemp("", "vl-spec-*.json")
	if err != nil {
		return false, err
	}
	tmpJSONPath := tmpJSON.Name()
	defer os.Remove(tmpJSONPath)
	if _, err := tmpJSON.Write(jsonBytes); err != nil {
		tmpJSON.Close()
		return false, err
	}
	tmpJSON.Close()

	// Render.
	code, renderErr := r.VLConvert.RenderSVG(context.Background(), tmpJSONPath, svgPath)
	if renderErr != nil || code != 0 {
		errMsg := ""
		if renderErr != nil {
			errMsg = renderErr.Error()
		}
		excerpt := errMsg
		if len(excerpt) > 200 {
			excerpt = excerpt[:200]
		}
		fmt.Fprintf(stdout, "CHART|RENDER|%s|%s\n", fence.ID, excerpt)
		_ = WriteFailed(assetsDir, fence.ID, errMsg)
		return false, nil
	}

	// Read SVG back (VLConvert wrote it to svgPath).
	svgBytes, err := os.ReadFile(svgPath)
	if err != nil {
		return false, err
	}

	// Write JSON sidecar and hash (SVG already at svgPath).
	if err := os.WriteFile(filepath.Join(assetsDir, fence.ID+".json"), jsonBytes, 0644); err != nil {
		return false, err
	}
	if err := os.WriteFile(filepath.Join(assetsDir, fence.ID+".svg.hash"), []byte(newHash), 0644); err != nil {
		return false, err
	}
	_ = os.Remove(filepath.Join(assetsDir, fence.ID+".svg.failed"))
	_ = svgBytes // SVG was written by vl-convert directly.

	fmt.Fprintf(stdout, "CHART|rendered %s\n", fence.ID)

	// Inject preview region.
	newText := InjectPreviewAt(pageText, fence.ID, r.RepoRoot, assetsDir, pagePath)
	if newText != pageText {
		if err := atomicWrite(pagePath, newText); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func cleanOrphans(r *Runner, assetsDir string, knownIDs map[string]struct{}, stdout io.Writer) error {
	removed, err := Cleanup(assetsDir, knownIDs)
	for _, path := range removed {
		fmt.Fprintf(stdout, "CHART|removed orphan %s\n", path)
	}
	if len(removed) > 0 {
		// For each removed chart, strip its preview region from all content pages.
		removedIDs := map[string]struct{}{}
		for _, path := range removed {
			name := filepath.Base(path)
			id := extractChartID(name, []string{".svg.hash", ".svg.failed", ".svg", ".json"})
			if id != "" {
				removedIDs[id] = struct{}{}
			}
		}
		_ = filepath.WalkDir(r.ContentDir, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil || d.IsDir() || filepath.Ext(path) != ".md" {
				return nil
			}
			b, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			text := string(b)
			newText := text
			for id := range removedIDs {
				newText = RemovePreview(newText, id)
			}
			if newText != text {
				_ = atomicWrite(path, newText)
			}
			return nil
		})
	}
	return err
}

// atomicWrite writes content to path atomically (write to temp, then rename).
func atomicWrite(path, content string) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".awiki-chart-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, path)
}
