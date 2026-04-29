package dataset_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"awiki/internal/dataset"
)

func makeInitRunner(t *testing.T) (*dataset.Runner, string) {
	t.Helper()
	root := t.TempDir()
	r := &dataset.Runner{
		RepoRoot:   root,
		ContentDir: filepath.Join(root, "content"),
		DataDir:    filepath.Join(root, "data"),
	}
	return r, root
}

// setupTemplates creates the scripts/templates directory with a minimal
// wiki-data-layer.md template for testing.
func setupTemplates(t *testing.T, root string) {
	t.Helper()
	tmplDir := filepath.Join(root, "scripts", "templates")
	if err := os.MkdirAll(tmplDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tmpl := "<!-- BEGIN data-layer -->\n## Data Layer\nSome content.\n<!-- END data-layer -->\n"
	if err := os.WriteFile(filepath.Join(tmplDir, "wiki-data-layer.md"), []byte(tmpl), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDataInit_FreshRun(t *testing.T) {
	r, root := makeInitRunner(t)
	setupTemplates(t, root)
	// Create a WIKI.md so step_wiki_md can find it.
	wikiPath := filepath.Join(root, "WIKI.md")
	if err := os.WriteFile(wikiPath, []byte("# Wiki\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if err := r.DataInit(&stdout, &stderr); err != nil {
		t.Fatalf("unexpected error: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}

	out := stdout.String()

	// Dirs should be created.
	for _, d := range []string{"content/datasets", "content/queries", "content/charts", "data", "assets/charts", "static/vendor/vega"} {
		if _, err := os.Stat(filepath.Join(root, d)); err != nil {
			t.Errorf("expected dir %s to exist: %v", d, err)
		}
		if !strings.Contains(out, "DATA-INIT|created "+d) && !strings.Contains(out, "DATA-INIT|skip "+d) {
			// Either created or skip is acceptable depending on preexisting state.
		}
	}

	// Config should be populated.
	cfgPath := filepath.Join(root, ".awiki", "config")
	cfg, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("config not created: %v", err)
	}
	cfgStr := string(cfg)
	for _, kv := range []string{"AWIKI_DATA_LAYER=on", "AWIKI_DATASET_INLINE_MAX_ROWS=500"} {
		if !strings.Contains(cfgStr, kv) {
			t.Errorf("expected %s in config, got:\n%s", kv, cfgStr)
		}
	}

	// Encryption skip record should be in stderr.
	if !strings.Contains(stderr.String(), "DATA-INIT|encryption-skipped") {
		t.Errorf("expected encryption-skipped in stderr, got: %s", stderr.String())
	}

	// WIKI.md should be patched.
	wiki, _ := os.ReadFile(wikiPath)
	if !strings.Contains(string(wiki), "data-layer") {
		t.Errorf("expected data-layer in WIKI.md, got:\n%s", string(wiki))
	}
}

func TestDataInit_SecondRunIsNoOp(t *testing.T) {
	r, root := makeInitRunner(t)
	setupTemplates(t, root)
	wikiPath := filepath.Join(root, "WIKI.md")
	if err := os.WriteFile(wikiPath, []byte("# Wiki\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout1, stderr1 bytes.Buffer
	if err := r.DataInit(&stdout1, &stderr1); err != nil {
		t.Fatalf("first run error: %v", err)
	}

	var stdout2, stderr2 bytes.Buffer
	if err := r.DataInit(&stdout2, &stderr2); err != nil {
		t.Fatalf("second run error: %v", err)
	}

	out2 := stdout2.String()

	// All dirs should be skipped.
	for _, d := range []string{"content/datasets", "content/queries"} {
		if !strings.Contains(out2, "DATA-INIT|skip "+d+" (exists)") {
			t.Errorf("expected skip for %s on second run, stdout:\n%s", d, out2)
		}
	}

	// All config keys should be skipped.
	if !strings.Contains(out2, "DATA-INIT|skip AWIKI_DATA_LAYER (already set)") {
		t.Errorf("expected config key skip on second run, stdout:\n%s", out2)
	}
}

func TestDataInit_MissingTemplateHandledGracefully(t *testing.T) {
	r, root := makeInitRunner(t)
	// No template directory set up.
	wikiPath := filepath.Join(root, "WIKI.md")
	if err := os.WriteFile(wikiPath, []byte("# Wiki\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	// Should not return an error — missing template is a warn, not a fatal.
	if err := r.DataInit(&stdout, &stderr); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(stderr.String(), "DATA-INIT|WARN") {
		t.Errorf("expected warn in stderr for missing template, got: %s", stderr.String())
	}
}
