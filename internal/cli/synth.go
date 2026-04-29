package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"awiki/internal/adapters"
	"awiki/internal/config"
	"awiki/internal/synth"
)

var synthSlugRegexp = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// runSynth dispatches `awiki synth <verb> [args]`.
func runSynth(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: awiki synth <verb> [args]")
		return 1
	}
	verb, rest := args[0], args[1:]
	r, err := buildSynthRunner()
	if err != nil {
		fmt.Fprintf(stderr, "synth: %v\n", err)
		return 1
	}
	switch verb {
	case "list":
		return runSynthList(r, rest, stdout, stderr)
	case "resolve":
		return runSynthResolve(r, rest, stdout, stderr)
	case "refine":
		return runSynthRefine(r, rest, stderr)
	case "new", "regen", "accept-stage", "finalize":
		fmt.Fprintf(stderr, "synth: verb %q not yet ported\n", verb)
		return 1
	default:
		fmt.Fprintf(stderr, "synth: unknown verb %q\n", verb)
		return 1
	}
}

func buildSynthRunner() (*synth.Runner, error) {
	repoRoot := os.Getenv("AWIKI_REPO_ROOT")
	if repoRoot == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		repoRoot = wd
	}
	pluginDir := os.Getenv("AWIKI_SYNTH_PLUGINS_DIR")
	if pluginDir == "" {
		pluginDir = filepath.Join(repoRoot, "synthesis-plugins")
	}
	cfg, _ := config.Load(filepath.Join(repoRoot, ".awiki", "config"))
	return &synth.Runner{
		RepoRoot:   repoRoot,
		ContentDir: filepath.Join(repoRoot, "content"),
		PluginDir:  pluginDir,
		Config:     cfg,
		Qmd:        adapters.ExecQmd{},
		PostHook:   adapters.ExecPostHook{},
		Git:        adapters.ExecSynthGit{},
		Lint:       adapters.ExecSynthLint{},
	}, nil
}

func runSynthResolve(r *synth.Runner, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("synth resolve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(stderr, "usage: awiki synth resolve <slug>")
		return 1
	}
	slug := fs.Arg(0)
	if !synthSlugRegexp.MatchString(slug) {
		fmt.Fprintf(stderr, "synth resolve: invalid slug %q (must match [a-z0-9][a-z0-9-]*)\n", slug)
		return 1
	}
	if err := r.Resolve(context.Background(), slug, stdout); err != nil {
		fmt.Fprintf(stderr, "synth resolve: %v\n", err)
		return 2
	}
	return 0
}

func runSynthRefine(r *synth.Runner, args []string, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage: awiki synth refine <slug> <note...>")
		return 1
	}
	slug := args[0]
	if !synthSlugRegexp.MatchString(slug) {
		fmt.Fprintf(stderr, "synth refine: invalid slug %q (must match [a-z0-9][a-z0-9-]*)\n", slug)
		return 1
	}
	note := strings.Join(args[1:], " ")
	if err := r.Refine(slug, note, stderr); err != nil {
		fmt.Fprintf(stderr, "synth refine: %v\n", err)
		return 2
	}
	return 0
}

func runSynthList(r *synth.Runner, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("synth list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if err := r.List(stdout); err != nil {
		fmt.Fprintf(stderr, "synth list: %v\n", err)
		if errors.Is(err, synth.ErrNoPlugins) {
			return 1
		}
		return 2
	}
	return 0
}
