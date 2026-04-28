package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"awiki/internal/lint"
)

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: awiki <command> [args]")
		return 1
	}
	switch args[0] {
	case "lint":
		return runLint(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		return 1
	}
}

func runLint(args []string, stdout io.Writer, stderr io.Writer) int {
	opts, err := parseLintOptions(args, stderr)
	if err != nil {
		return 1
	}
	collector, code := lint.Run(opts)
	for _, fix := range collector.Fixes {
		fmt.Fprintln(stdout, fix.Record())
	}
	for _, d := range collector.Diagnostics {
		fmt.Fprintln(stdout, d.Record())
	}
	fmt.Fprintln(stdout, collector.Summary())
	return code
}

func parseLintOptions(args []string, stderr io.Writer) (lint.Options, error) {
	fs := flag.NewFlagSet("lint", flag.ContinueOnError)
	fs.SetOutput(stderr)
	opts := lint.Options{ContentDir: "content", RepoRoot: "."}
	fs.BoolVar(&opts.Fix, "fix", false, "apply mechanical fixes")
	fs.StringVar(&opts.Only, "only", "", "run only a lint namespace")
	fs.StringVar(&opts.OnlyFile, "file", "", "run against one file")
	fs.BoolVar(&opts.HugoCheck, "hugo-check", false, "run Hugo render check")
	fs.BoolVar(&opts.AliasBuildOnly, "alias-build-only", false, "build maps only")
	var flagArgs []string
	contentDirSet := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if len(arg) > 0 && arg[0] == '-' {
			flagArgs = append(flagArgs, arg)
			if isLintStringFlag(arg) && !hasInlineFlagValue(arg) && i+1 < len(args) {
				i++
				flagArgs = append(flagArgs, args[i])
			}
			continue
		}
		if !contentDirSet {
			opts.ContentDir = arg
			contentDirSet = true
		} else {
			flagArgs = append(flagArgs, arg)
		}
	}
	if err := fs.Parse(flagArgs); err != nil {
		return opts, err
	}
	opts.RepoRoot = inferLintRepoRoot(opts.ContentDir, opts.RepoRoot)
	return opts, nil
}

func inferLintRepoRoot(contentDir string, fallback string) string {
	cleanContent := filepath.Clean(contentDir)
	if filepath.Base(cleanContent) != "content" {
		return fallback
	}
	parent := filepath.Dir(cleanContent)
	if _, err := os.Stat(filepath.Join(parent, "scripts", "lint.sh")); err != nil {
		return fallback
	}
	return parent
}

func isLintStringFlag(arg string) bool {
	return arg == "-only" || arg == "--only" || arg == "-file" || arg == "--file"
}

func hasInlineFlagValue(arg string) bool {
	for _, r := range arg {
		if r == '=' {
			return true
		}
	}
	return false
}
