package ops

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func allPresent(name string) (bool, error) { return true, nil }

func TestCheckDepsAllPresent(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := CheckDeps(CheckDepsOptions{
		OS:          "Darwin",
		BashVersion: "5.2.21",
		Looker:      allPresent,
		PyImport:    func(string) bool { return true },
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "DEPS-SUMMARY|errors=0") {
		t.Fatalf("missing summary: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "OK|git\n") {
		t.Fatalf("missing OK|git: %q", stdout.String())
	}
}

func TestCheckDepsFakeMissingGit(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := CheckDeps(CheckDepsOptions{
		OS:          "Darwin",
		BashVersion: "5.2.21",
		FakeMissing: "git",
		Looker:      allPresent,
		PyImport:    func(string) bool { return true },
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "MISSING|git|min=2.30") {
		t.Fatalf("missing record: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "DEPS-SUMMARY|errors=1") {
		t.Fatalf("missing summary: %q", stderr.String())
	}
}

func TestCheckDepsFakeMissingFlock(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := CheckDeps(CheckDepsOptions{
		OS:          "Darwin",
		BashVersion: "5.2.21",
		FakeMissing: "flock",
		Looker:      allPresent,
		PyImport:    func(string) bool { return true },
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected 1, got %d", code)
	}
	combined := stdout.String() + "\n" + stderr.String()
	if !strings.Contains(combined, "flock") {
		t.Fatalf("missing flock in output: %s", combined)
	}
	if !strings.Contains(combined, "util-linux") {
		t.Fatalf("missing util-linux hint: %s", combined)
	}
}

func TestCheckDepsBashLessThan4(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := CheckDeps(CheckDepsOptions{
		OS:          "Linux",
		BashVersion: "3.2.57",
		Looker:      allPresent,
		PyImport:    func(string) bool { return true },
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "MISSING|bash|need=4+") {
		t.Fatalf("missing bash record: %q", stderr.String())
	}
}

func TestCheckDepsPythonCalamineFakeMissing(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := CheckDeps(CheckDepsOptions{
		OS:          "Darwin",
		BashVersion: "5.2.21",
		FakeMissing: "python_calamine",
		Looker:      allPresent,
		PyImport:    func(string) bool { return true },
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "OPTIONAL-MISSING|python-calamine") {
		t.Fatalf("missing record: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "pip3 install python-calamine") {
		t.Fatalf("missing hint: %q", stderr.String())
	}
}

func TestCheckDepsDataLayerWarning(t *testing.T) {
	tmp := t.TempDir()
	cfg := filepath.Join(tmp, ".awiki", "config")
	if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte("AWIKI_DATA_LAYER=on\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	// Looker returns false for vl-convert/duckdb, true for python3.
	look := func(n string) (bool, error) {
		switch n {
		case "vl-convert", "duckdb":
			return false, nil
		}
		return true, nil
	}
	code := CheckDeps(CheckDepsOptions{
		OS:          "Darwin",
		BashVersion: "5.2.21",
		RepoRoot:    tmp,
		Looker:      look,
		PyImport:    func(string) bool { return true },
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "data layer is on") {
		t.Fatalf("missing advisory: %q", stdout.String())
	}
}
