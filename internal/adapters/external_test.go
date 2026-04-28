package adapters

import (
	"context"
	"reflect"
	"testing"
)

type fakeRunner struct {
	name string
	args []string
}

func (r *fakeRunner) Run(_ context.Context, name string, args ...string) (string, int, error) {
	r.name = name
	r.args = append([]string(nil), args...)
	return "", 0, nil
}

func TestLegacyLintBuildsLegacyCommand(t *testing.T) {
	runner := &fakeRunner{}

	_, _, err := LegacyLint(context.Background(), runner, "/repo", "synth", "content/synthesis/demo.md", "content", true)
	if err != nil {
		t.Fatalf("LegacyLint() error = %v", err)
	}

	if runner.name != "env" {
		t.Fatalf("command name = %q, want env", runner.name)
	}
	want := []string{
		"AWIKI_LINT_LEGACY=1",
		"bash",
		"scripts/lint.sh",
		"--only=synth",
		"--fix",
		"--file=content/synthesis/demo.md",
		"content",
	}
	if !reflect.DeepEqual(runner.args, want) {
		t.Fatalf("args = %#v, want %#v", runner.args, want)
	}
}

func TestHugoCheckBuildsRenderToMemoryCommand(t *testing.T) {
	runner := &fakeRunner{}

	_, _, err := HugoCheck(context.Background(), runner)
	if err != nil {
		t.Fatalf("HugoCheck() error = %v", err)
	}

	if runner.name != "hugo" {
		t.Fatalf("command name = %q, want hugo", runner.name)
	}
	want := []string{"--source", ".", "--renderToMemory", "--logLevel", "error"}
	if !reflect.DeepEqual(runner.args, want) {
		t.Fatalf("args = %#v, want %#v", runner.args, want)
	}
}
