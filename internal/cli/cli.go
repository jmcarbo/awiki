package cli

import (
	"flag"
	"fmt"
	"io"

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
	fs := flag.NewFlagSet("lint", flag.ContinueOnError)
	fs.SetOutput(stderr)
	opts := lint.Options{ContentDir: "content", RepoRoot: "."}
	fs.BoolVar(&opts.Fix, "fix", false, "apply mechanical fixes")
	fs.StringVar(&opts.Only, "only", "", "run only a lint namespace")
	fs.StringVar(&opts.OnlyFile, "file", "", "run against one file")
	fs.BoolVar(&opts.HugoCheck, "hugo-check", false, "run Hugo render check")
	fs.BoolVar(&opts.AliasBuildOnly, "alias-build-only", false, "build maps only")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() > 0 {
		opts.ContentDir = fs.Arg(0)
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
