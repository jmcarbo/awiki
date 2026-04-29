package dataset_test

import (
	"os"
	"path/filepath"
	"testing"

	"awiki/internal/dataset"
)

func TestInlineThresholds_Defaults(t *testing.T) {
	// No .awiki/config file → defaults.
	maxRows, maxBytes := dataset.InlineThresholds(t.TempDir())
	if maxRows != 500 {
		t.Errorf("default maxRows: want 500, got %d", maxRows)
	}
	if maxBytes != 51200 {
		t.Errorf("default maxBytes: want 51200, got %d", maxBytes)
	}
}

func TestInlineThresholds_Override(t *testing.T) {
	root := t.TempDir()
	awikiDir := filepath.Join(root, ".awiki")
	if err := os.MkdirAll(awikiDir, 0755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(awikiDir, "config")
	if err := os.WriteFile(cfgPath, []byte("AWIKI_DATASET_INLINE_MAX_ROWS=100\nAWIKI_DATASET_INLINE_MAX_BYTES=1024\n"), 0644); err != nil {
		t.Fatal(err)
	}

	maxRows, maxBytes := dataset.InlineThresholds(root)
	if maxRows != 100 {
		t.Errorf("want 100, got %d", maxRows)
	}
	if maxBytes != 1024 {
		t.Errorf("want 1024, got %d", maxBytes)
	}
}

func TestInlineThresholds_PartialOverride(t *testing.T) {
	root := t.TempDir()
	awikiDir := filepath.Join(root, ".awiki")
	if err := os.MkdirAll(awikiDir, 0755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(awikiDir, "config")
	// Only override rows; bytes should fall back to default.
	if err := os.WriteFile(cfgPath, []byte("AWIKI_DATASET_INLINE_MAX_ROWS=200\n"), 0644); err != nil {
		t.Fatal(err)
	}

	maxRows, maxBytes := dataset.InlineThresholds(root)
	if maxRows != 200 {
		t.Errorf("want 200, got %d", maxRows)
	}
	if maxBytes != 51200 {
		t.Errorf("bytes should be default 51200, got %d", maxBytes)
	}
}

func TestInlineThresholds_InvalidValues(t *testing.T) {
	root := t.TempDir()
	awikiDir := filepath.Join(root, ".awiki")
	if err := os.MkdirAll(awikiDir, 0755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(awikiDir, "config")
	if err := os.WriteFile(cfgPath, []byte("AWIKI_DATASET_INLINE_MAX_ROWS=notanumber\nAWIKI_DATASET_INLINE_MAX_BYTES=also_bad\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Invalid values → fall back to defaults.
	maxRows, maxBytes := dataset.InlineThresholds(root)
	if maxRows != 500 {
		t.Errorf("invalid value should fall back to 500, got %d", maxRows)
	}
	if maxBytes != 51200 {
		t.Errorf("invalid value should fall back to 51200, got %d", maxBytes)
	}
}
