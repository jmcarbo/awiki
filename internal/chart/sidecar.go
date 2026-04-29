package chart

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteSidecars writes the resolved spec JSON, the SVG bytes, and the hash
// file to <dir>/<id>.{json,svg,svg.hash}. It also removes any stale
// <id>.svg.failed marker left from a previous failure.
func WriteSidecars(dir, id string, resolvedSpec map[string]any, svg []byte) error {
	jsonBytes, err := json.MarshalIndent(resolvedSpec, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal resolved spec: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".json"), jsonBytes, 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, id+".svg"), svg, 0644); err != nil {
		return err
	}
	hash := Hash(resolvedSpec)
	if err := os.WriteFile(filepath.Join(dir, id+".svg.hash"), []byte(hash), 0644); err != nil {
		return err
	}
	// Remove any previous failure marker.
	_ = os.Remove(filepath.Join(dir, id+".svg.failed"))
	return nil
}

// ReadHash reads the stored hash for a chart sidecar. ok=false when the hash
// file does not exist.
func ReadHash(dir, id string) (string, bool) {
	b, err := os.ReadFile(filepath.Join(dir, id+".svg.hash"))
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(b)), true
}

// WriteFailed writes a truncated error message (max 200 chars) to
// <dir>/<id>.svg.failed.
func WriteFailed(dir, id, errMsg string) error {
	if len(errMsg) > 200 {
		errMsg = errMsg[:200]
	}
	return os.WriteFile(filepath.Join(dir, id+".svg.failed"), []byte(errMsg), 0644)
}

// Cleanup removes sidecar files for chart IDs not present in knownIDs.
// Extensions cleaned: .svg, .svg.hash, .svg.failed, .json.
// Returns the list of removed file paths and any first error encountered.
func Cleanup(dir string, knownIDs map[string]struct{}) (removed []string, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	extensions := []string{".svg.hash", ".svg.failed", ".svg", ".json"}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		id := extractChartID(name, extensions)
		if id == "" {
			continue
		}
		if _, keep := knownIDs[id]; keep {
			continue
		}
		path := filepath.Join(dir, name)
		if removeErr := os.Remove(path); removeErr != nil && err == nil {
			err = removeErr
		} else {
			removed = append(removed, path)
		}
	}
	return removed, err
}

// extractChartID strips known sidecar extensions from a filename and returns
// the bare chart ID. Returns "" if the file does not have a known extension.
func extractChartID(name string, extensions []string) string {
	for _, ext := range extensions {
		if strings.HasSuffix(name, ext) {
			return name[:len(name)-len(ext)]
		}
	}
	return ""
}
