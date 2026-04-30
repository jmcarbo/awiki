package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"awiki/internal/adapters"
	"awiki/internal/template"
)

// runTemplate dispatches `awiki template <verb> [args...]`. The verbs
// covered here mirror the Python helpers ported in internal/template/:
// init, status, gc, source-check, manifest, attr-audit, config, plan,
// provenance, lint. Each verb is a thin driver around the helper —
// callers that still want bash semantics can invoke the bash shim.
func runTemplate(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: awiki template <verb> [args...]")
		return 2
	}
	verb := args[0]
	rest := args[1:]
	switch verb {
	case "status":
		return runTemplateStatus(rest, stdout, stderr)
	case "gc":
		return runTemplateGC(rest, stdout, stderr)
	case "source-check":
		return runTemplateSourceCheck(rest, stdout, stderr)
	case "manifest":
		return runTemplateManifest(rest, stdout, stderr)
	case "config":
		return runTemplateConfig(rest, stdout, stderr)
	case "provenance":
		return runTemplateProvenance(rest, stdout, stderr)
	case "lint":
		return runTemplateLint(rest, stdout, stderr)
	case "escape":
		return runTemplateEscape(rest, stdout, stderr)
	case "retrofit":
		return runTemplateRetrofit(rest, stdout, stderr)
	case "merge":
		return runTemplateMerge(rest, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "template: unknown verb %q\n", verb)
		return 2
	}
}

// templateRepoRoot returns the resolved repo root used by the template
// verbs. It honours AWIKI_REPO_ROOT and falls back to the current
// directory.
func templateRepoRoot() string {
	if env := os.Getenv("AWIKI_REPO_ROOT"); env != "" {
		if abs, err := filepath.Abs(env); err == nil {
			return abs
		}
		return env
	}
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// --- status -----------------------------------------------------------------

func runTemplateStatus(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("template status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", "", "repo root (defaults to AWIKI_REPO_ROOT or cwd)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	repoRoot := *root
	if repoRoot == "" {
		repoRoot = templateRepoRoot()
	}
	pj := filepath.Join(repoRoot, ".awiki", "template.json")
	if _, err := os.Stat(pj); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(stderr, "halt: no .awiki/template.json. Run 'just template-init' or 'just template-retrofit'.")
			return 1
		}
		fmt.Fprintln(stderr, err)
		return 1
	}
	prov, err := template.LoadProvenance(pj)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "version: %s\n", prov.Version)
	fmt.Fprintf(stdout, "commit:  %s\n", prov.Commit)
	fmt.Fprintf(stdout, "repo:    %s\n", prov.Repo)
	if prov.OriginalRepo != "" && prov.Repo != prov.OriginalRepo {
		fmt.Fprintf(stdout, "original_repo: %s  (DIFFERS — source was changed)\n", prov.OriginalRepo)
	}
	pp := filepath.Join(repoRoot, ".awiki", "pending-prompts")
	if entries, err := os.ReadDir(pp); err == nil && len(entries) > 0 {
		fmt.Fprintln(stdout, "pending prompts:")
		for _, e := range entries {
			fmt.Fprintln(stdout, e.Name())
		}
	}
	state := filepath.Join(repoRoot, ".awiki", "template-cache", "_fetch", ".update-state.json")
	if _, err := os.Stat(state); err == nil {
		phase, _ := template.GetStateField(state, "phase")
		if phase == "" {
			phase = "?"
		}
		fmt.Fprintf(stdout, "in-progress update: phase=%s\n", phase)
	}
	return 0
}

// --- gc ---------------------------------------------------------------------

func runTemplateGC(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("template gc", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", "", "repo root")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	repoRoot := *root
	if repoRoot == "" {
		repoRoot = templateRepoRoot()
	}
	pj := filepath.Join(repoRoot, ".awiki", "template.json")
	prov, err := template.LoadProvenance(pj)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	cacheDir := filepath.Join(repoRoot, ".awiki", "template-cache")
	if err := template.RotateCache(cacheDir, prov.Commit); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, "info: cache GC complete (kept current + previous)")
	return 0
}

// --- source-check -----------------------------------------------------------

func runTemplateSourceCheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("template source-check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	provenance := fs.String("provenance", "", ".awiki/template.json path")
	source := fs.String("source", "", "candidate source URL or path")
	accept := fs.Bool("accept-source-change", false, "accept the change and continue")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *provenance == "" || !fileExists(*provenance) {
		fmt.Fprintf(stderr, "provenance not found: %s\n", *provenance)
		return 2
	}
	if *source == "" {
		fmt.Fprintln(stderr, "--source required")
		return 2
	}
	prov, err := template.LoadProvenance(*provenance)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	pinned := prov.Repo
	if pinned == *source {
		return 0
	}
	if *accept {
		fmt.Fprintf(stderr, "info: source change accepted (pinned=%s, new=%s)\n", pinned, *source)
		return 0
	}
	fmt.Fprintf(stderr, "Source change detected:\n  pinned : %s\n  new    : %s\nThis will execute migrations and overwrite tracked files from the new source.\nRe-run with --accept-source-change to proceed.\n", pinned, *source)
	return 1
}

// --- manifest ---------------------------------------------------------------

func runTemplateManifest(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage: awiki template manifest <subcmd> <path> [args...]")
		return 2
	}
	subcmd := args[0]
	path := args[1]
	if !fileExists(path) {
		fmt.Fprintf(stderr, "manifest not found: %s\n", path)
		return 1
	}
	m, err := template.LoadManifest(path)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	switch subcmd {
	case "load":
		_, _ = io.WriteString(stdout, m.FormatLoad())
		return 0
	case "resolve":
		if len(args) < 3 {
			fmt.Fprintln(stderr, "usage: resolve <manifest> <relpath>")
			return 2
		}
		fmt.Fprintln(stdout, m.ResolveStrategy(args[2]))
		return 0
	case "bootstrap-ids":
		for _, id := range m.Bootstrap.OrderedSteps {
			fmt.Fprintln(stdout, id)
		}
		return 0
	case "dangerous-ids":
		for _, id := range m.Bootstrap.Dangerous.IDs {
			fmt.Fprintln(stdout, id)
		}
		return 0
	case "has-glob-overlap":
		strat, glob, ok := m.HasGlobOverlap()
		if ok {
			fmt.Fprintf(stderr, "duplicate glob in %s: %s\n", strat, glob)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(stderr, "unknown subcmd: %s\n", subcmd)
		return 2
	}
}

// --- config -----------------------------------------------------------------

// runTemplateConfig mirrors scripts/template-config.sh. Two subcommands:
// `get <path> <key> [default]` and `validate <path>`.
func runTemplateConfig(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage:\n  awiki template config get <path> <key> [default]\n  awiki template config validate <path>")
		return 2
	}
	subcmd := args[0]
	path := args[1]
	switch subcmd {
	case "get":
		if len(args) < 3 {
			fmt.Fprintln(stderr, "usage: get <path> <key> [default]")
			return 2
		}
		key := args[2]
		def := ""
		if len(args) >= 4 {
			def = args[3]
		}
		val := configGet(path, key, def)
		fmt.Fprintln(stdout, val)
		return 0
	case "validate":
		return configValidate(path, stderr)
	default:
		fmt.Fprintf(stderr, "unknown subcmd: %s\n", subcmd)
		return 2
	}
}

var configKnownKeys = map[string]bool{
	"default_branch":         true,
	"no_template_check":      true,
	"require_signature":      true,
	"ingest_lint_threshold":  true,
}

func configGet(path, key, def string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return def
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}
		if line[:idx] == key {
			val := line[idx+1:]
			if val == "" {
				return def
			}
			return val
		}
	}
	return def
}

func configValidate(path string, stderr io.Writer) int {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0
		}
		fmt.Fprintln(stderr, err)
		return 1
	}
	rc := 0
	lineno := 0
	for _, raw := range strings.Split(string(data), "\n") {
		lineno++
		// Drop the synthetic empty trailing line produced by a final
		// "\n" — bash `while read` would not iterate it.
		if lineno == strings.Count(string(data), "\n")+1 && raw == "" {
			break
		}
		trim := strings.TrimSpace(raw)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		// Same regex as bash: ^[A-Za-z_][A-Za-z0-9_]*=.*$
		if !validConfigLine(raw) {
			fmt.Fprintf(stderr, "config error: malformed line %d: %s\n", lineno, raw)
			rc = 1
			continue
		}
		key := raw[:strings.Index(raw, "=")]
		if !configKnownKeys[key] {
			fmt.Fprintf(stderr, "config warning: unknown key %s (line %d)\n", key, lineno)
		}
	}
	return rc
}

func validConfigLine(line string) bool {
	idx := strings.Index(line, "=")
	if idx <= 0 {
		return false
	}
	key := line[:idx]
	if key == "" {
		return false
	}
	c := key[0]
	if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_') {
		return false
	}
	for i := 1; i < len(key); i++ {
		c := key[i]
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

// --- provenance -------------------------------------------------------------

func runTemplateProvenance(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage: awiki template provenance <subcmd> <path> [args...]")
		return 2
	}
	subcmd := args[0]
	path := args[1]
	rest := args[2:]
	switch subcmd {
	case "init":
		if len(rest) < 4 {
			fmt.Fprintln(stderr, "usage: init <path> <repo> <ref> <version> <commit>")
			return 2
		}
		if err := template.InitProvenance(path, rest[0], rest[1], rest[2], rest[3]); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	case "get":
		if len(rest) < 1 {
			fmt.Fprintln(stderr, "usage: get <path> <field>")
			return 2
		}
		val, err := template.GetProvenanceField(path, rest[0])
		if err != nil {
			fmt.Fprintf(stderr, "field not found: %s\n", rest[0])
			return 1
		}
		fmt.Fprintln(stdout, val)
		return 0
	case "set":
		if len(rest) < 2 {
			fmt.Fprintln(stderr, "usage: set <path> <field> <value>")
			return 2
		}
		if err := template.SetProvenanceField(path, rest[0], rest[1]); err != nil {
			if errors.Is(err, template.ErrImmutableField) {
				fmt.Fprintf(stderr, "field is immutable: %s\n", rest[0])
				return 1
			}
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	case "append-migration":
		if len(rest) < 2 {
			fmt.Fprintln(stderr, "usage: append-migration <path> <id> <status> [reason]")
			return 2
		}
		reason := ""
		if len(rest) >= 3 {
			reason = rest[2]
		}
		if err := template.AppendProvenanceMigration(path, rest[0], rest[1], reason); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	case "append-bootstrap-step":
		if len(rest) < 2 {
			fmt.Fprintln(stderr, "usage: append-bootstrap-step <path> <id> <status> [reason] [content_hash]")
			return 2
		}
		reason, hash := "", ""
		if len(rest) >= 3 {
			reason = rest[2]
		}
		if len(rest) >= 4 {
			hash = rest[3]
		}
		if err := template.AppendProvenanceBootstrapStep(path, rest[0], rest[1], reason, hash); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	case "has-step-applied":
		if len(rest) < 2 {
			fmt.Fprintln(stderr, "usage: has-step-applied <path> <id> <expected_hash>")
			return 2
		}
		ok, err := template.HasStepApplied(path, rest[0], rest[1])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if ok {
			return 0
		}
		return 1
	case "list-steps":
		out, err := template.ListProvenanceSteps(path)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		_, _ = io.WriteString(stdout, out)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown subcmd: %s\n", subcmd)
		return 2
	}
}

// --- lint -------------------------------------------------------------------

func runTemplateLint(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("template lint", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", "", "repo root")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	repoRoot := *root
	if repoRoot == "" {
		repoRoot = templateRepoRoot()
	}
	report, err := template.LintTemplate(repoRoot, time.Now(), os.Getenv("AWIKI_NO_TEMPLATE_CHECK"), nil)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	for _, m := range report.Messages {
		fmt.Fprintln(stdout, m.Format())
	}
	return report.ExitCode()
}

// --- escape -----------------------------------------------------------------

func runTemplateEscape(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: awiki template escape <string>")
		return 2
	}
	fmt.Fprintln(stdout, template.EscapePlanField(args[0]))
	return 0
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// --- retrofit ---------------------------------------------------------------

func runTemplateRetrofit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("template retrofit", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repo := fs.String("repo", "", "template source URL or path")
	ref := fs.String("ref", "main", "template ref (default main)")
	version := fs.String("version", "", "template version")
	commit := fs.String("commit", "", "template commit SHA")
	nonInteractive := fs.Bool("non-interactive", false, "skip interactive step confirmation")
	heuristicsOnly := fs.Bool("heuristics-only", false, "print heuristic detections only; do not seed")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	root := templateRepoRoot()
	g := adapters.TemplateOrchGit{RepoRoot: root, Stdout: stdout, Stderr: stderr}

	res, err := template.Retrofit(g, template.RetrofitInput{
		RepoRoot:       root,
		Repo:           *repo,
		Ref:            *ref,
		Version:        *version,
		Commit:         *commit,
		NonInteractive: *nonInteractive,
		HeuristicsOnly: *heuristicsOnly,
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if res.AlreadyHasProvenance {
		pj := filepath.Join(root, ".awiki", "template.json")
		fmt.Fprintf(stdout, "info: %s already exists; nothing to retrofit\n", pj)
		return 0
	}
	for _, l := range res.HeuristicLines {
		fmt.Fprintln(stdout, l)
	}
	if res.InitRan {
		fmt.Fprintln(stdout, "info: retrofit complete")
	}
	return 0
}

// --- merge ------------------------------------------------------------------

func runTemplateMerge(args []string, stdout, stderr io.Writer) int {
	if len(args) != 3 {
		fmt.Fprintln(stderr, "usage: awiki template merge <cur> <base> <new>")
		return 2
	}
	cur, base, newFile := args[0], args[1], args[2]
	g := adapters.TemplateOrchGit{Stdout: stdout, Stderr: stderr}
	rc, err := template.MergeFiles(g, cur, base, newFile)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 3
	}
	return rc
}

// --- bootstrap-step (top-level verb) ----------------------------------------

// runBootstrapStep mirrors `awiki bootstrap-step <id>` (alias for
// `template-update.sh --rerun-bootstrap-step <id>`). The driver is thin
// — it resolves the repo root + default branch, then delegates to
// template.RerunBootstrapStep with adapters.
func runBootstrapStep(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("bootstrap-step", flag.ContinueOnError)
	fs.SetOutput(stderr)
	nonInteractive := fs.Bool("non-interactive", false, "auto-confirm the replay (CI mode)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	rest := fs.Args()
	if len(rest) != 1 {
		fmt.Fprintln(stderr, "usage: awiki bootstrap-step <id>")
		return 2
	}
	stepID := rest[0]

	repoRoot := templateRepoRoot()
	defaultBranch := configGet(filepath.Join(repoRoot, ".awiki", "config"), "default_branch", "main")

	g := adapters.TemplateOrchGit{RepoRoot: repoRoot, Stdout: stdout, Stderr: stderr}
	b := adapters.TemplateOrchBash{Stdout: stdout, Stderr: stderr}
	c := adapters.TemplateOrchConfirm{In: os.Stdin, Out: stderr}

	res, err := template.RerunBootstrapStep(g, b, c, template.BootstrapStepInput{
		RepoRoot:       repoRoot,
		StepID:         stepID,
		NonInteractive: *nonInteractive,
		DefaultBranch:  defaultBranch,
		PathEnv:        os.Getenv("PATH"),
		HomeEnv:        os.Getenv("HOME"),
		LangEnv:        os.Getenv("LANG"),
		LCAllEnv:       os.Getenv("LC_ALL"),
	})
	if err != nil {
		switch {
		case errors.Is(err, template.ErrUpdateInProgress):
			fmt.Fprintln(stderr, "halt: update in progress — run --abort first")
		case errors.Is(err, template.ErrPendingPrompts):
			fmt.Fprintln(stderr, "halt: pending-prompts present — resolve first")
		case errors.Is(err, template.ErrUpdateBranchPresent):
			fmt.Fprintln(stderr, "halt: existing update branch present — finish or --abort first")
		case errors.Is(err, template.ErrStepNotFound):
			fmt.Fprintf(stderr, "step not found: %s\n", stepID)
		default:
			fmt.Fprintln(stderr, err)
		}
		return 1
	}
	if res.Declined {
		// Bash exits 0 silently; nothing else to print.
		return 0
	}
	// Surface the body to the user (matches the `echo "$BODY"` in bash).
	if !*nonInteractive {
		// In interactive mode the confirm prompt already printed the body.
	} else {
		fmt.Fprintln(stdout, res.StepBody)
	}
	fmt.Fprintf(stdout, "info: rerun-bootstrap-step %s complete on %s\n", stepID, res.Branch)
	return 0
}
