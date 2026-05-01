package initverb

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- Step 12: template-init ---

func TestStepTemplateInitSkipsWithoutManifest(t *testing.T) {
	dir := t.TempDir()
	ctx := StepContext{
		Ctx: context.Background(), RepoRoot: dir,
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
	}
	res, err := (stepTemplateInit{}).Execute(ctx, &Answers{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != StatusSkipped {
		t.Fatalf("expected skipped, got %+v", res)
	}
}

func TestStepTemplateInitSeedsProvenance(t *testing.T) {
	dir := t.TempDir()
	// Manifest with one bootstrap step we can hash.
	manifest := `schema_version = 1
template_version = "1.0.0"

[strategies]
overwrite = ["**"]

[new_file_default]
strategy = "prompt"

[bootstrap]
ordered_steps = ["dep-check"]

[bootstrap.dangerous]
ids = []
`
	if err := os.WriteFile(filepath.Join(dir, "template.manifest.toml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	bs := `# BOOTSTRAP

### Step 0
<!-- bootstrap-step: dep-check -->

Run dep-check.
`
	if err := os.WriteFile(filepath.Join(dir, "BOOTSTRAP.md"), []byte(bs), 0o644); err != nil {
		t.Fatal(err)
	}
	// State with one applied step so we can verify provenance.
	state := NewState(time.Now(), "claude")
	state.UpsertStep("dep-check", StatusApplied, "ok", time.Now())
	if err := SaveState(dir, state); err != nil {
		t.Fatal(err)
	}
	git := &fakeGit{revParse: "deadbeefcafef00d"}
	ctx := StepContext{
		Ctx: context.Background(), RepoRoot: dir,
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
		Git:    git,
	}
	res, err := (stepTemplateInit{}).Execute(ctx, &Answers{})
	if err != nil {
		t.Fatalf("template-init: %v", err)
	}
	if res.Status != StatusApplied {
		t.Fatalf("status=%s", res.Status)
	}
	pj := filepath.Join(dir, ".awiki", "template.json")
	if _, err := os.Stat(pj); err != nil {
		t.Fatalf("template.json: %v", err)
	}
	// Verify provenance has dep-check recorded with a sha256 hash.
	body, _ := os.ReadFile(pj)
	if !strings.Contains(string(body), `"dep-check"`) {
		t.Errorf("template.json missing dep-check entry: %s", body)
	}
	if !strings.Contains(string(body), "sha256:") {
		t.Errorf("template.json missing content_hash: %s", body)
	}
}

// --- Step 13: smoke-test ---

func TestStepSmokeTestPrints(t *testing.T) {
	stdout := &bytes.Buffer{}
	ctx := StepContext{Stdout: stdout, Stderr: &bytes.Buffer{}}
	res, err := (stepSmokeTest{}).Execute(ctx, &Answers{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != StatusApplied {
		t.Fatalf("status=%s", res.Status)
	}
	out := stdout.String()
	for _, want := range []string{"smoke test", "just ingest", "just lint", "just serve"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in stdout: %s", want, out)
		}
	}
}
