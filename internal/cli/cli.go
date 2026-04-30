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
	case "synth":
		return runSynth(args[1:], stdout, stderr)
	case "dataset":
		return runDataset(args[1:], stdout, stderr)
	case "data-init":
		return runDataInit(args[1:], stdout, stderr)
	case "chart":
		return runChart(args[1:], stdout, stderr)
	case "query":
		return runQuery(args[1:], stdout, stderr)
	case "ingest",
		"ingest-xlsx",
		"ingest-git",
		"ingest-pdf",
		"ingest-audio",
		"capture",
		"watchdog",
		"ingest-batch-list",
		"ingest-git-list":
		return runIngestVerb(args[0], args[1:], stdout, stderr)
	case "template":
		return runTemplate(args[1:], stdout, stderr)
	case "bootstrap-step":
		return runBootstrapStep(args[1:], stdout, stderr)
	case "log":
		return runLog(args[1:], stdout, stderr)
	case "reindex":
		return runReindex(args[1:], stdout, stderr)
	case "check-deps":
		return runCheckDeps(args[1:], stdout, stderr)
	case "rename":
		return runRename(args[1:], stdout, stderr)
	case "delete":
		return runDelete(args[1:], stdout, stderr)
	case "update-catalog":
		return runUpdateCatalog(args[1:], stdout, stderr)
	case "scan":
		return runScan(args[1:], stdout, stderr)
	case "recur":
		return runRecur(false, args[1:], stdout, stderr)
	case "recur-dry":
		return runRecur(true, args[1:], stdout, stderr)
	case "agenda":
		return runAgenda(args[1:], stdout, stderr)
	case "review":
		return runReview(args[1:], stdout, stderr)
	case "triage-apply":
		return runTriageApply(args[1:], stdout, stderr)
	case "triage":
		return runTriage(args[1:], os.Stdin, stdout, stderr)
	case "build":
		return runBuild(args[1:], stdout, stderr)
	case "serve":
		return runServe(args[1:], stdout, stderr)
	case "install-hooks":
		return runInstallHooks(args[1:], stdout, stderr)
	case "install-qmd":
		return runInstallQmd(args[1:], stdout, stderr)
	case "wire-awiki-mcp":
		return runWireAwikiMCP(args[1:], stdout, stderr)
	case "wire-qmd-mcp":
		return runWireQmdMCP(args[1:], stdout, stderr)
	case "task-init":
		return runTaskInit(args[1:], stdout, stderr)
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
	opts := lint.Options{ContentDir: "content", RepoRoot: ".", ToolRoot: inferLintToolRoot(".")}
	if envRoot := os.Getenv("AWIKI_REPO_ROOT"); envRoot != "" {
		opts.RepoRoot = envRoot
		opts.ContentDir = filepath.Join(envRoot, "content")
	}
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
	if !hasLintScript(parent) {
		return fallback
	}
	if abs, err := filepath.Abs(parent); err == nil {
		return abs
	}
	return parent
}

func inferLintToolRoot(fallback string) string {
	if hasLintScript(fallback) {
		if abs, err := filepath.Abs(fallback); err == nil {
			return abs
		}
		return fallback
	}
	exe, err := os.Executable()
	if err == nil {
		candidate := filepath.Dir(filepath.Dir(exe))
		if hasLintScript(candidate) {
			if abs, err := filepath.Abs(candidate); err == nil {
				return abs
			}
			return candidate
		}
	}
	return fallback
}

func hasLintScript(root string) bool {
	_, err := os.Stat(filepath.Join(root, "scripts", "lint.sh"))
	return err == nil
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
