package synth

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListEmitsTabSeparatedRecordsSortedByName(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "synthesis-plugins")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	plugins := map[string]string{
		"zeta":  "Z description",
		"alpha": "A description",
		"mid":   "M description",
	}
	for name, desc := range plugins {
		body := "---\nname: " + name +
			"\nversion: 1\noutput_type: synthesis\noutput_subtype: " + name +
			"\ndescription: " + desc + "\n---\nbody\n"
		if err := os.WriteFile(filepath.Join(pluginDir, name+".md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r := &Runner{RepoRoot: dir, PluginDir: pluginDir}
	var out bytes.Buffer
	if err := r.List(&out); err != nil {
		t.Fatalf("List: %v", err)
	}
	got := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	want := []string{
		"alpha\tsynthesis\talpha\tA description",
		"mid\tsynthesis\tmid\tM description",
		"zeta\tsynthesis\tzeta\tZ description",
	}
	if len(got) != len(want) {
		t.Fatalf("len: got %d want %d (out=%q)", len(got), len(want), out.String())
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("line %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestListErrorsWhenDirAbsent(t *testing.T) {
	dir := t.TempDir()
	r := &Runner{RepoRoot: dir, PluginDir: filepath.Join(dir, "missing")}
	if err := r.List(&bytes.Buffer{}); !errors.Is(err, ErrNoPlugins) {
		t.Fatalf("got %v, want ErrNoPlugins", err)
	}
}

func TestListErrorsWhenDirEmpty(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "synthesis-plugins")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	r := &Runner{RepoRoot: dir, PluginDir: pluginDir}
	if err := r.List(&bytes.Buffer{}); !errors.Is(err, ErrNoPlugins) {
		t.Fatalf("got %v, want ErrNoPlugins", err)
	}
}
