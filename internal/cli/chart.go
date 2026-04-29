package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"awiki/internal/adapters"
	"awiki/internal/chart"
)

func runChart(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: awiki chart <new|render|render-one> [args]")
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
	contentDir := filepath.Join(repoRoot, "content")
	assetsDir := filepath.Join(repoRoot, "assets", "charts")

	runner := &chart.Runner{
		RepoRoot:   repoRoot,
		ContentDir: contentDir,
		AssetsDir:  assetsDir,
		VLConvert:  adapters.ExecVLConvert{},
		VendorVega: adapters.ExecVendorVega{},
	}

	switch verb {
	case "new":
		return runChartNew(runner, rest, stdout, stderr)
	case "render":
		return runChartRender(runner, rest, stdout, stderr)
	case "render-one":
		return runChartRenderOne(runner, rest, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "CHART|ERROR|unknown subcommand: %s\n", verb)
		return 1
	}
}

func runChartNew(runner *chart.Runner, args []string, stdout io.Writer, stderr io.Writer) int {
	opts := chart.NewOptions{}
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--data="):
			opts.DataSlug = strings.TrimPrefix(arg, "--data=")
		case strings.HasPrefix(arg, "-"):
			fmt.Fprintf(stderr, "CHART|ERROR|unknown flag: %s\n", arg)
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

func runChartRender(runner *chart.Runner, args []string, stdout io.Writer, stderr io.Writer) int {
	opts := chart.RenderOptions{}
	for _, arg := range args {
		if arg == "--keep-orphans" {
			opts.KeepOrphans = true
		}
	}
	if err := runner.Render(opts, stdout, stderr); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	return 0
}

func runChartRenderOne(runner *chart.Runner, args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "CHART|ERROR|usage: chart render-one <chart-id>")
		return 1
	}
	if err := runner.RenderOne(args[0], stdout, stderr); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	return 0
}
