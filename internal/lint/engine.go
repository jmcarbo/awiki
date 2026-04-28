package lint

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"awiki/internal/adapters"
	"awiki/internal/wiki"
)

func Run(opts Options) (Collector, int) {
	var c Collector
	hasCustomRunner := opts.Runner != nil
	runner := opts.Runner
	if runner == nil {
		runner = adapters.ExecRunner{}
	}
	if opts.ToolRoot == "" {
		opts.ToolRoot = opts.RepoRoot
	}

	if opts.AliasBuildOnly {
		args := []string{"AWIKI_LINT_LEGACY=1"}
		if opts.RepoRoot != "" {
			args = append(args, "AWIKI_REPO_ROOT="+opts.RepoRoot)
		}
		args = append(args, "bash", "scripts/lint.sh", "--alias-build-only", opts.ContentDir)
		output, code, _ := runner.RunInDir(context.Background(), opts.ToolRoot, "env", args...)
		imported := importExternalRecords(&c, output)
		addLegacyFailureDiagnostic(&c, "alias-build", code, imported)
		return c, c.ExitCode()
	}

	if opts.Only != "" && opts.Only != "all" {
		if isDeferredNamespace(opts.Only) {
			runLegacyNamespace(&c, runner, opts, opts.Only)
			return c, c.ExitCode()
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
	if hasCustomRunner || hasLegacyLintScript(opts.ToolRoot) {
		runDeferredLegacyLint(&c, runner, opts)
	}
	if opts.HugoCheck {
		runHugoCheck(&c, runner, opts.RepoRoot)
	}
	return c, c.ExitCode()
}

func hasLegacyLintScript(toolRoot string) bool {
	if toolRoot == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(toolRoot, "scripts", "lint.sh"))
	return err == nil
}

func isDeferredNamespace(only string) bool {
	for _, namespace := range deferredNamespaces() {
		if only == namespace {
			return true
		}
	}
	return false
}

func runDeferredLegacyLint(c *Collector, runner adapters.Runner, opts Options) {
	for _, namespace := range deferredNamespaces() {
		runLegacyNamespace(c, runner, opts, namespace)
	}
}

func deferredNamespaces() []string {
	return []string{"synth", "data", "chart", "task", "query"}
}

func runLegacyNamespace(c *Collector, runner adapters.Runner, opts Options, namespace string) {
	output, code, _ := adapters.LegacyLint(context.Background(), runner, opts.ToolRoot, opts.RepoRoot, namespace, opts.OnlyFile, opts.ContentDir, opts.Fix)
	imported := importExternalRecords(c, output)
	addLegacyFailureDiagnostic(c, namespace, code, imported)
}

func addLegacyFailureDiagnostic(c *Collector, namespace string, code int, importedLintDiagnostics int) {
	if code == 0 || importedLintDiagnostics > 0 {
		return
	}
	c.Add(Diagnostic{
		Level:   Error,
		File:    namespace,
		Message: fmt.Sprintf("legacy lint failed with exit code %d", code),
	})
}

func runHugoCheck(c *Collector, runner adapters.Runner, repoRoot string) {
	_, code, err := adapters.HugoCheck(context.Background(), runner, repoRoot)
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

func importExternalRecords(c *Collector, output string) int {
	importedLintDiagnostics := 0
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "LINT|") {
			if importDiagnostic(c, line) {
				importedLintDiagnostics++
			}
			continue
		}
		if strings.HasPrefix(line, "FIX|") {
			importFix(c, line)
		}
	}
	return importedLintDiagnostics
}

func importDiagnostic(c *Collector, line string) bool {
	parts := strings.SplitN(line, "|", 5)
	if len(parts) != 4 && len(parts) != 5 {
		return false
	}
	levelText := strings.ToUpper(parts[1])
	if levelText == "WARNING" {
		levelText = string(Warn)
	}
	level := Level(levelText)
	switch level {
	case Error, Warn, Info:
	default:
		return false
	}
	diagnostic := Diagnostic{Level: level, File: parts[2], Message: parts[3]}
	if len(parts) == 5 {
		diagnostic.Code = parts[3]
		diagnostic.Message = parts[4]
	}
	c.Add(diagnostic)
	return true
}

func importFix(c *Collector, line string) {
	parts := strings.SplitN(line, "|", 3)
	if len(parts) != 3 {
		return
	}
	c.AddFix(FixRecord{File: parts[1], Message: parts[2]})
}
