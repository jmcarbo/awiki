package template

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeManifestForReplay(t *testing.T, dangerous []string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "manifest.toml")
	dangerousList := ""
	for i, d := range dangerous {
		if i > 0 {
			dangerousList += ", "
		}
		dangerousList += `"` + d + `"`
	}
	body := `schema_version = 1
template_version = "0.0.0"
[strategies]
overwrite = []
preserve = []
three_way = []
attributes_merge = []
template_only = []
[new_file_default]
strategy = "prompt"
[bootstrap]
ordered_steps = ["dep-check", "domain", "stage-commit"]
[bootstrap.dangerous]
ids = [` + dangerousList + `]
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestClassifyBootstrapStep_OK(t *testing.T) {
	bp := writeBootstrap(t, fixtureBootstrapMD)
	mp := writeManifestForReplay(t, nil)
	c, err := ClassifyBootstrapStep(bp, mp, "dep-check")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if c != BootstrapClassOK {
		t.Errorf("c=%q want ok", c)
	}
}

func TestClassifyBootstrapStep_Dangerous(t *testing.T) {
	bp := writeBootstrap(t, fixtureBootstrapMD)
	mp := writeManifestForReplay(t, []string{"stage-commit"})
	c, err := ClassifyBootstrapStep(bp, mp, "stage-commit")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if c != BootstrapClassDangerous {
		t.Errorf("c=%q want dangerous", c)
	}
}

func TestClassifyBootstrapStep_Missing(t *testing.T) {
	bp := writeBootstrap(t, fixtureBootstrapMD)
	mp := writeManifestForReplay(t, nil)
	c, err := ClassifyBootstrapStep(bp, mp, "nonexistent")
	if c != BootstrapClassMissing {
		t.Errorf("c=%q want missing", c)
	}
	if !errors.Is(err, ErrStepNotFound) {
		t.Errorf("err=%v want ErrStepNotFound", err)
	}
	if !IsBootstrapMissing(err) {
		t.Errorf("IsBootstrapMissing returned false for %v", err)
	}
}

func TestReplayBootstrapStepBody(t *testing.T) {
	bp := writeBootstrap(t, fixtureBootstrapMD)
	body, err := ReplayBootstrapStepBody(bp, "domain")
	if err != nil {
		t.Fatalf("Body: %v", err)
	}
	if !strings.Contains(body, "personal | research") {
		t.Errorf("body=%q missing expected content", body)
	}
}

func TestReplayBootstrapStepBody_Missing(t *testing.T) {
	bp := writeBootstrap(t, fixtureBootstrapMD)
	_, err := ReplayBootstrapStepBody(bp, "nonexistent")
	if !errors.Is(err, ErrStepNotFound) {
		t.Errorf("err=%v want ErrStepNotFound", err)
	}
}
