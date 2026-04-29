package dataset

import (
	"path/filepath"
	"strconv"

	"awiki/internal/config"
)

const (
	defaultInlineMaxRows  = 500
	defaultInlineMaxBytes = 51200
)

// InlineThresholds reads .awiki/config under repoRoot and returns (maxRows,
// maxBytes) from AWIKI_DATASET_INLINE_MAX_ROWS and
// AWIKI_DATASET_INLINE_MAX_BYTES. Falls back to (500, 51200) when the keys
// are absent or the file does not exist.
func InlineThresholds(repoRoot string) (maxRows, maxBytes int) {
	maxRows = defaultInlineMaxRows
	maxBytes = defaultInlineMaxBytes

	cfg, _ := config.Load(filepath.Join(repoRoot, ".awiki", "config"))

	if s, ok := cfg["AWIKI_DATASET_INLINE_MAX_ROWS"]; ok {
		if n, err := strconv.Atoi(s); err == nil {
			maxRows = n
		}
	}
	if s, ok := cfg["AWIKI_DATASET_INLINE_MAX_BYTES"]; ok {
		if n, err := strconv.Atoi(s); err == nil {
			maxBytes = n
		}
	}
	return maxRows, maxBytes
}
