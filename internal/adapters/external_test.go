package adapters

import (
	"context"
	"reflect"
	"testing"
)

type fakeRunner struct {
	dir  string
	name string
	args []string
}

func (r *fakeRunner) Run(_ context.Context, name string, args ...string) (string, int, error) {
	return r.RunInDir(context.Background(), "", name, args...)
}

func (r *fakeRunner) RunInDir(_ context.Context, dir string, name string, args ...string) (string, int, error) {
	r.dir = dir
	r.name = name
	r.args = append([]string(nil), args...)
	return "", 0, nil
}

func TestLegacyLintRunsFromToolRootViaAwikiBinary(t *testing.T) {
	runner := &fakeRunner{}

	_, _, err := LegacyLint(context.Background(), runner, "/tool", "/wiki", "synth", "content/synthesis/demo.md", "content", true)
	if err != nil {
		t.Fatalf("LegacyLint() error = %v", err)
	}

	if runner.name != "env" {
		t.Fatalf("command name = %q, want env", runner.name)
	}
	if runner.dir != "/tool" {
		t.Fatalf("command dir = %q, want /tool", runner.dir)
	}
	if len(runner.args) < 6 {
		t.Fatalf("args too short: %#v", runner.args)
	}
	// args[0] is the AWIKI_REPO_ROOT env. args[1] is the resolved
	// awiki binary path (varies per host). The remaining args must
	// match the lint verb invocation byte-for-byte.
	if runner.args[0] != "AWIKI_REPO_ROOT=/wiki" {
		t.Fatalf("args[0] = %q, want AWIKI_REPO_ROOT=/wiki", runner.args[0])
	}
	tail := runner.args[2:]
	want := []string{
		"lint",
		"--only=synth",
		"--fix",
		"--file=content/synthesis/demo.md",
		"content",
	}
	if !reflect.DeepEqual(tail, want) {
		t.Fatalf("args[2:] = %#v, want %#v", tail, want)
	}
}

func TestHugoCheckBuildsRenderToMemoryCommand(t *testing.T) {
	runner := &fakeRunner{}

	_, _, err := HugoCheck(context.Background(), runner, "/wiki")
	if err != nil {
		t.Fatalf("HugoCheck() error = %v", err)
	}

	if runner.name != "hugo" {
		t.Fatalf("command name = %q, want hugo", runner.name)
	}
	if runner.dir != "/wiki" {
		t.Fatalf("command dir = %q, want /wiki", runner.dir)
	}
	want := []string{"--source", ".", "--renderToMemory", "--logLevel", "error"}
	if !reflect.DeepEqual(runner.args, want) {
		t.Fatalf("args = %#v, want %#v", runner.args, want)
	}
}
