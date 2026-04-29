package synth

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestListEmitsSortedPlugins(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "plugins", "synth")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"zeta", "alpha", "mid"} {
		body := "---\nname: " + name + "\nversion: 1\nmin_sources: 2\nmax_sources: 8\n" +
			"required_sections: [Summary, Sources]\n---\nbody\n"
		if err := os.WriteFile(filepath.Join(pluginDir, name+".md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r := &Runner{RepoRoot: dir, PluginDir: pluginDir}
	var out bytes.Buffer
	if err := r.List(&out); err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	want := []string{
		"PLUGIN|alpha|version=1|min=2|max=8|sections=Summary,Sources",
		"PLUGIN|mid|version=1|min=2|max=8|sections=Summary,Sources",
		"PLUGIN|zeta|version=1|min=2|max=8|sections=Summary,Sources",
	}
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("len: got %d want %d (out=%q)", len(got), len(want), out.String())
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("line %d: got %q want %q", i, got[i], want[i])
		}
	}
}
