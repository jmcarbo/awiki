package lint

import (
	"context"
	"strings"

	"awiki/internal/adapters"
	"awiki/internal/wiki"
)

func Run(opts Options) (Collector, int) {
	var c Collector
	runner := opts.Runner
	if runner == nil {
		runner = adapters.ExecRunner{}
	}

	if opts.AliasBuildOnly {
		output, code, _ := runner.Run(context.Background(), "env", "AWIKI_LINT_LEGACY=1", "bash", "scripts/lint.sh", "--alias-build-only", opts.ContentDir)
		importExternalRecords(&c, output)
		return c, externalExitCode(c, code)
	}

	if opts.Only != "" && opts.Only != "all" {
		if isDeferredNamespace(opts.Only) {
			output, code, _ := adapters.LegacyLint(context.Background(), runner, opts.RepoRoot, opts.Only, opts.OnlyFile, opts.ContentDir, opts.Fix)
			importExternalRecords(&c, output)
			return c, externalExitCode(c, code)
		}
		return c, c.ExitCode()
	}

	pages, err := wiki.DiscoverPages(opts.ContentDir)
	if err != nil {
		c.Add(Diagnostic{
			Level:   Error,
			File:    opts.ContentDir,
			Message: err.Error(),
		})
		return c, 2
	}

	if opts.Fix {
		applyLastUpdatedFixes(&c, opts, pages)
		pages, err = wiki.DiscoverPages(opts.ContentDir)
		if err != nil {
			c.Add(Diagnostic{
				Level:   Error,
				File:    opts.ContentDir,
				Message: err.Error(),
			})
			return c, 2
		}
	}

	idx := wiki.BuildIndex(pages)
	runCoreRules(&c, idx)
	if opts.HugoCheck {
		runHugoCheck(&c, runner)
	}
	return c, c.ExitCode()
}

func isDeferredNamespace(only string) bool {
	switch only {
	case "synth", "data", "chart", "task", "query":
		return true
	default:
		return false
	}
}

func runHugoCheck(c *Collector, runner adapters.Runner) {
	_, code, err := adapters.HugoCheck(context.Background(), runner)
	if err == nil && code == 0 {
		return
	}
	if code == 127 {
		c.Add(Diagnostic{
			Level:   Info,
			File:    "hugo",
			Message: "--hugo-check requested but hugo not on PATH; skipping",
		})
		return
	}
	c.Add(Diagnostic{
		Level:   Error,
		File:    "hugo",
		Message: "template render failed (run 'just build' for details)",
	})
}

func externalExitCode(c Collector, code int) int {
	if code != 0 {
		return code
	}
	return c.ExitCode()
}

func importExternalRecords(c *Collector, output string) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "LINT|") {
			importDiagnostic(c, line)
			continue
		}
		if strings.HasPrefix(line, "FIX|") {
			importFix(c, line)
		}
	}
}

func importDiagnostic(c *Collector, line string) {
	parts := strings.SplitN(line, "|", 4)
	if len(parts) != 4 {
		return
	}
	level := Level(parts[1])
	switch level {
	case Error, Warn, Info:
	default:
		return
	}
	c.Add(Diagnostic{Level: level, File: parts[2], Message: parts[3]})
}

func importFix(c *Collector, line string) {
	parts := strings.SplitN(line, "|", 3)
	if len(parts) != 3 {
		return
	}
	c.AddFix(FixRecord{File: parts[1], Message: parts[2]})
}
