package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadParsesKeyValueLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	body := "# comment\nAWIKI_AGENT=claude\n\nDATASET_INLINE_THRESHOLD=100\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got["AWIKI_AGENT"] != "claude" {
		t.Fatalf("AWIKI_AGENT: %q", got["AWIKI_AGENT"])
	}
	if got["DATASET_INLINE_THRESHOLD"] != "100" {
		t.Fatalf("DATASET_INLINE_THRESHOLD: %q", got["DATASET_INLINE_THRESHOLD"])
	}
}

func TestLoadMissingFileReturnsEmptyMap(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestLoadStripsInlineComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte("KEY=value  # trailing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got["KEY"] != "value" {
		t.Fatalf("KEY: %q", got["KEY"])
	}
}
