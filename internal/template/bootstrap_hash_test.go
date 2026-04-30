package template

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixtureBootstrapMD = `# Bootstrap

### Step 0. Dependency check
<!-- bootstrap-step: dep-check -->
Run ` + "`bash scripts/check-deps.sh`" + `. Halt on missing required tools.

### Step 1. Domain
<!-- bootstrap-step: domain -->
Ask user: which domain (personal | research | book | business | other)?

### Step 11. Stage initial commit
<!-- bootstrap-step: stage-commit -->
` + "`git add . && git commit -m \"init: bootstrapped from awiki vX.Y.Z\"`" + `.
`

func writeBootstrap(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "BOOTSTRAP.md")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestListBootstrapStepIDs(t *testing.T) {
	ids := ListBootstrapStepIDs(fixtureBootstrapMD)
	want := []string{"dep-check", "domain", "stage-commit"}
	if len(ids) != len(want) {
		t.Fatalf("ids=%v want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Errorf("ids[%d]=%q want %q", i, ids[i], want[i])
		}
	}
}

func TestParseBootstrapBodies_DelimitsAtNextHeading(t *testing.T) {
	bodies := ParseBootstrapBodies(fixtureBootstrapMD)
	if _, ok := bodies["dep-check"]; !ok {
		t.Fatal("dep-check missing")
	}
	if !strings.Contains(bodies["dep-check"], "check-deps.sh") {
		t.Errorf("dep-check body missing expected content")
	}
	if strings.Contains(bodies["dep-check"], "Domain") {
		t.Errorf("dep-check body bled into next step")
	}
	if !strings.Contains(bodies["domain"], "personal | research") {
		t.Errorf("domain body missing expected content")
	}
}

func TestHashBootstrapStep_PrefixAndLength(t *testing.T) {
	h, err := HashBootstrapStep(fixtureBootstrapMD, "dep-check")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if !strings.HasPrefix(h, "sha256:") {
		t.Errorf("h=%q must start with sha256:", h)
	}
	if len(h) != 71 { // "sha256:" + 64 hex
		t.Errorf("len=%d want 71", len(h))
	}
}

func TestHashBootstrapStep_WhitespaceNormalized(t *testing.T) {
	a := `### Step 1.
<!-- bootstrap-step: x -->
hello world

`
	b := `### Step 1.
<!-- bootstrap-step: x -->
   hello   world
`
	ha, err := HashBootstrapStep(a, "x")
	if err != nil {
		t.Fatalf("Hash a: %v", err)
	}
	hb, err := HashBootstrapStep(b, "x")
	if err != nil {
		t.Fatalf("Hash b: %v", err)
	}
	if ha != hb {
		t.Errorf("ha=%q hb=%q want equal", ha, hb)
	}
}

func TestHashBootstrapStep_MissingStep(t *testing.T) {
	_, err := HashBootstrapStep(fixtureBootstrapMD, "nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrStepNotFound) {
		t.Errorf("err=%v want ErrStepNotFound", err)
	}
}

func TestHashBootstrapStep_DifferentStepsDiffer(t *testing.T) {
	a, err := HashBootstrapStep(fixtureBootstrapMD, "dep-check")
	if err != nil {
		t.Fatalf("hash a: %v", err)
	}
	b, err := HashBootstrapStep(fixtureBootstrapMD, "domain")
	if err != nil {
		t.Fatalf("hash b: %v", err)
	}
	if a == b {
		t.Errorf("expected distinct hashes for distinct steps")
	}
}

func TestBootstrapStepBody_RawNoNormalization(t *testing.T) {
	body, err := BootstrapStepBody(fixtureBootstrapMD, "dep-check")
	if err != nil {
		t.Fatalf("Body: %v", err)
	}
	// Raw body must include the leading newline that follows the
	// marker comment (Python's md[start:end] is identical).
	if !strings.HasPrefix(body, "\n") {
		t.Errorf("expected body to start with newline, got %q", body[:min(20, len(body))])
	}
}

func TestHashBootstrapStepFile(t *testing.T) {
	p := writeBootstrap(t, fixtureBootstrapMD)
	h, err := HashBootstrapStepFile(p, "dep-check")
	if err != nil {
		t.Fatalf("HashFile: %v", err)
	}
	if !strings.HasPrefix(h, "sha256:") {
		t.Errorf("h=%q", h)
	}
}

func TestListBootstrapStepIDsFile_MissingFile(t *testing.T) {
	_, err := ListBootstrapStepIDsFile("/definitely/missing/BOOTSTRAP.md")
	if err == nil {
		t.Error("expected error for missing file")
	}
}
