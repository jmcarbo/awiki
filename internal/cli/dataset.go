package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"awiki/internal/config"
	"awiki/internal/dataset"
)

func runDataset(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: awiki dataset <verb> [args]")
		return 1
	}
	verb, rest := args[0], args[1:]
	r, err := buildDatasetRunner()
	if err != nil {
		fmt.Fprintf(stderr, "dataset: %v\n", err)
		return 1
	}
	switch verb {
	case "validate":
		return runDatasetValidate(r, rest, stdout, stderr)
	case "compact":
		return runDatasetCompact(r, rest, stdout, stderr)
	case "new":
		return runDatasetNew(r, rest, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "dataset: unknown verb %q\n", verb)
		return 1
	}
}

func buildDatasetRunner() (*dataset.Runner, error) {
	repoRoot := os.Getenv("AWIKI_REPO_ROOT")
	if repoRoot == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		repoRoot = wd
	}
	cfg, _ := config.Load(filepath.Join(repoRoot, ".awiki", "config"))
	return &dataset.Runner{
		RepoRoot:   repoRoot,
		ContentDir: filepath.Join(repoRoot, "content"),
		DataDir:    filepath.Join(repoRoot, "data"),
		Config:     cfg,
	}, nil
}

func runDatasetValidate(r *dataset.Runner, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("dataset validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(stderr, "usage: awiki dataset validate <slug>")
		return 1
	}
	if err := r.Validate(fs.Arg(0), stdout, stderr); err != nil {
		fmt.Fprintf(stderr, "dataset validate: %v\n", err)
		return 1
	}
	return 0
}

func runDatasetCompact(r *dataset.Runner, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("dataset compact", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(stderr, "usage: awiki dataset compact <slug>")
		return 1
	}
	if err := r.Compact(fs.Arg(0), stdout); err != nil {
		fmt.Fprintf(stderr, "dataset compact: %v\n", err)
		return 1
	}
	return 0
}

func runDatasetNew(r *dataset.Runner, args []string, stdout, stderr io.Writer) int {
	var format, from string
	fs := flag.NewFlagSet("dataset new", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&format, "format", "csv", "data format (csv|tsv|json|dsv|topojson)")
	fs.StringVar(&from, "from", "", "source data file to embed")
	// Support --format=X and --from=X style (flag package handles these natively).
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(stderr, "usage: awiki dataset new <slug> [--format=csv] [--from=<file>]")
		return 1
	}
	slug := fs.Arg(0)
	if err := r.New(dataset.NewOptions{Slug: slug, Format: format, From: from}, stdout); err != nil {
		fmt.Fprintf(stderr, "dataset new: %v\n", err)
		return 1
	}
	return 0
}
