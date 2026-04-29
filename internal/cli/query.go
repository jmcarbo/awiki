package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"awiki/internal/adapters"
	"awiki/internal/query"
)

func runQuery(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: awiki query <run|new|render|render-one|fence-render> [args]")
		return 1
	}
	verb := args[0]
	rest := args[1:]

	repoRoot := os.Getenv("AWIKI_REPO_ROOT")
	if repoRoot == "" {
		repoRoot = "."
	}
	if abs, err := filepath.Abs(repoRoot); err == nil {
		repoRoot = abs
	}

	runner := &query.Runner{
		RepoRoot:   repoRoot,
		ContentDir: filepath.Join(repoRoot, "content"),
		DataDir:    filepath.Join(repoRoot, "data"),
		DuckDB:     adapters.ExecDuckDB{},
		Today:      todayDate(),
	}

	switch verb {
	case "run":
		return runQueryRun(runner, rest, stdout, stderr)
	case "new":
		return runQueryNew(runner, rest, stdout, stderr)
	case "render":
		return runQueryRender(runner, rest, stdout, stderr)
	case "render-one":
		return runQueryRenderOne(runner, rest, stdout, stderr)
	case "fence-render":
		return runQueryFenceRender(runner, rest, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "QUERY|ERROR|unknown subcommand: %s\n", verb)
		return 1
	}
}

func runQueryRun(runner *query.Runner, args []string, stdout io.Writer, stderr io.Writer) int {
	opts := query.RunOptions{}
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--out="):
			opts.Out = strings.TrimPrefix(arg, "--out=")
		case arg == "--force":
			opts.Force = true
		case arg == "--":
			// skip
		case strings.HasPrefix(arg, "-"):
			fmt.Fprintf(stderr, "QUERY|ERROR|unknown flag: %s\n", arg)
			return 1
		default:
			opts.SQL = arg
		}
	}
	if err := runner.Run(context.Background(), opts, stdout); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	return 0
}

func runQueryNew(runner *query.Runner, args []string, stdout io.Writer, stderr io.Writer) int {
	opts := query.NewOptions{}
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--out="):
			opts.OutSlug = strings.TrimPrefix(arg, "--out=")
		case arg == "--":
			// skip
		case strings.HasPrefix(arg, "-"):
			fmt.Fprintf(stderr, "QUERY|ERROR|unknown flag: %s\n", arg)
			return 1
		default:
			opts.Slug = arg
		}
	}
	if err := runner.New(opts, stdout); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	return 0
}

func runQueryRender(runner *query.Runner, args []string, stdout io.Writer, stderr io.Writer) int {
	_ = args
	if err := runner.Render(context.Background(), stdout); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	return 0
}

func runQueryRenderOne(runner *query.Runner, args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "QUERY|ERROR|usage: query render-one <slug>")
		return 1
	}
	if err := runner.RenderOne(context.Background(), args[0], stdout); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	return 0
}

func runQueryFenceRender(runner *query.Runner, args []string, stdout io.Writer, stderr io.Writer) int {
	_ = args
	if err := runner.FenceRender(context.Background(), stdout); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	return 0
}

func todayDate() string {
	// Use os.Getenv("TODAY") if set, otherwise fall back to time package.
	if v := os.Getenv("AWIKI_TODAY"); v != "" {
		return v
	}
	// Import time lazily at call site to keep init fast.
	return todayFromTime()
}
