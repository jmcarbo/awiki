package ingest

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// BookkeepOptions describes one `awiki ingest <path>` invocation. The
// option set mirrors the bash CLI of scripts/ingest.sh:
//
//   - SourcePath  : positional <path-under-raw/inbox/...>
//   - AgentCLI    : --agent <cli>  (or --agent=<cli>)
//                   default: $AWIKI_AGENT (resolved by the dispatcher)
//   - AgentFlags  : $AWIKI_AGENT_FLAGS — extra argv before the prompt
//
// All fields can be empty; defaults match bash semantics.
type BookkeepOptions struct {
	SourcePath string
	AgentCLI   string
	AgentFlags string
}

// modeRe captures the validation+mode regex applied to the source path.
// Mirrors scripts/ingest.sh:42 — the leading `^` anchor and the literal
// `raw/inbox/` prefix are required; `(.+)` ensures at least one
// character of relative subtree.
var modeRe = regexp.MustCompile(`^raw/inbox/(interactive|batch|checkpoint)/(.+)$`)

// agentPromptFormat is the byte-for-byte template emitted on the
// AGENT-PROMPT line. Pinned by golden fixture in slice 7. The
// bash form lives at scripts/ingest.sh:114.
//
// IMPORTANT: do not adjust whitespace, punctuation, or the literal
// backtick-escaped `just lint`. External agents and hooks consume this
// record and rely on its exact form (see slice plan §"AGENT-PROMPT
// drift" risk).
const agentPromptFormat = "Process the source at %s per WIKI.md §4.1 ingest workflow steps 3-9 (mode=%s). Read it, write content/sources/<slug>.md, update affected entity/concept/topic pages, update content/catalog.md, update section indexes if section purpose changed. Run `just lint` afterward."

// IngestSingle is the single-file ingest entry point used by the
// watchdog loop and any other in-process caller that already has a
// concrete source path. It is a thin wrapper around IngestBookkeep
// that fixes the SourcePath and lets the caller default the agent
// fields (most callers leave them empty). Watchdog never invokes the
// agent CLI directly — the orchestrator's batch-mode auto-agent path
// is reached only via `awiki ingest <path>`.
func (r *Runner) IngestSingle(ctx context.Context, path string, stdout, stderr io.Writer) error {
	return IngestBookkeep(ctx, r, BookkeepOptions{SourcePath: path}, stdout, stderr)
}

// IngestBookkeep ports the bash bookkeeping flow at scripts/ingest.sh
// to Go. Steps:
//
//  1. Validate the source path lies under
//     raw/inbox/{interactive,batch,checkpoint}/.
//  2. Reject missing source (exit 1) and already-processed dest (exit 3).
//  3. Move src -> raw/processed/<mode>/<rel>.
//  4. (TODO slice 7+ ops port) call log-append. Bash error-swallows
//     this call so absent log is not a parity violation.
//  5. Increment the per-repo counter at .awiki/ingest-count.
//  6. Emit INGEST-OK|src=...|dest=...|mode=...|count=...
//  7. When count >= AWIKI_LINT_AFTER_N (default 5): emit AUTO-LINT
//     record, run lint via the adapter, reset the counter.
//  8. Best-effort qmd reindex when AWIKI_QMD_STATUS != "missing" and
//     the qmd binary is on PATH.
//  9. Emit AGENT-PROMPT|<prompt> on stdout.
// 10. When AgentCLI is set and mode == "batch", invoke the configured
//     agent CLI via the adapter. AgentCLI on non-batch mode emits
//     AGENT-SKIP|reason=mode-needs-human and skips invocation.
// 11. Final exit selection follows bash precedence:
//     LINT_RC (4) > QMD_RC (5) > AGENT_RC (6) > 0.
//
// IngestBookkeep returns an *ExitError when any of the deferred exits
// fire so the CLI dispatcher surfaces the same byte and the same
// process exit code as bash.
func IngestBookkeep(ctx context.Context, r *Runner, opts BookkeepOptions, stdout, stderr io.Writer) error {
	if opts.SourcePath == "" {
		fmt.Fprintln(stderr, "usage: ingest.sh [--agent <cli>] <path-under-raw/inbox/>")
		return &ExitError{Code: 1, Msg: "usage"}
	}

	src := opts.SourcePath
	if filepath.IsAbs(src) {
		if rel, err := filepath.Rel(r.RepoRoot, src); err == nil && !strings.HasPrefix(rel, "..") {
			src = filepath.ToSlash(rel)
		}
	} else {
		src = filepath.ToSlash(filepath.Clean(src))
	}

	// Validate mode + capture <mode> and relative subtree.
	m := modeRe.FindStringSubmatch(src)
	if m == nil {
		fmt.Fprintln(stderr, "ERROR: path must be under raw/inbox/{interactive,batch,checkpoint}/")
		return &ExitError{Code: 2, Msg: "bad-mode"}
	}
	mode := m[1]
	rel := m[2]

	srcAbs := filepath.Join(r.RepoRoot, src)
	if info, err := os.Stat(srcAbs); err != nil || info.IsDir() {
		fmt.Fprintf(stderr, "ERROR: source not found: %s\n", src)
		return &ExitError{Code: 1, Msg: "not-found"}
	}

	dest := filepath.ToSlash(filepath.Join("raw", "processed", mode, rel))
	destAbs := filepath.Join(r.RepoRoot, dest)
	if info, err := os.Stat(destAbs); err == nil && !info.IsDir() {
		fmt.Fprintf(stderr, "ERROR: already processed: %s\n", dest)
		return &ExitError{Code: 3, Msg: "already-processed"}
	}

	destDir := filepath.Dir(destAbs)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("ingest: mkdir dest dir: %w", err)
	}
	if err := os.Rename(srcAbs, destAbs); err != nil {
		return fmt.Errorf("ingest: move src->dest: %w", err)
	}

	// Log-append parity: bash invokes
	//   bash $SCRIPT_DIR/log-append.sh ingest "$(basename $SRC) mode=$MODE"
	// best-effort at scripts/ingest.sh:66. The Go path mirrors the
	// argv shape via the LogAppend adapter; errors are swallowed (the
	// bash form has no `|| ...` either — `set -e` is not effective on
	// a backgrounded best-effort here, and missing log-append.sh would
	// crash bash). A future ops/log domain port will replace the
	// shell-out with a native writer.
	if r.LogAppend != nil {
		_ = r.LogAppend.Append(ctx, r.RepoRoot, "ingest", filepath.Base(src)+" mode="+mode)
	}

	// Counter under .awiki/ingest-count.
	awikiDir := filepath.Join(r.RepoRoot, ".awiki")
	if err := os.MkdirAll(awikiDir, 0o755); err != nil {
		return fmt.Errorf("ingest: mkdir .awiki: %w", err)
	}
	counterFile := filepath.Join(awikiDir, "ingest-count")
	count := 0
	if data, err := os.ReadFile(counterFile); err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
			count = n
		}
	}
	count++
	if err := os.WriteFile(counterFile, []byte(strconv.Itoa(count)+"\n"), 0o644); err != nil {
		return fmt.Errorf("ingest: write counter: %w", err)
	}
	// Note: bash form writes via `echo "$COUNT" > "$COUNTER_FILE"`,
	// which produces "<n>\n" — we mirror byte-for-byte.

	fmt.Fprintf(stdout, "INGEST-OK|src=%s|dest=%s|mode=%s|count=%d\n", src, dest, mode, count)

	// Auto-lint at threshold.
	threshold := 5
	if v := envFromConfig(r, "AWIKI_LINT_AFTER_N"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			threshold = n
		}
	}

	lintRC := 0
	if count >= threshold {
		fmt.Fprintf(stdout, "AUTO-LINT|threshold=%d|count=%d\n", threshold, count)
		var code int
		var err error
		if r.Lint != nil {
			// Pass nil env to inherit; the adapter consults the
			// process env for AWIKI_LINT_CMD.
			code, err = r.Lint.Run(ctx, r.RepoRoot, nil)
		} else {
			// No adapter wired (test scenario or partial wiring).
			// Skip — bash form still resets the counter.
			code, err = 0, nil
		}
		_ = err // bash captures via `|| LINT_EXIT=$?`; non-zero handled below
		if code >= 2 {
			lintRC = 4
		}
		// Reset counter regardless of lint exit (matches bash:97).
		if err := os.WriteFile(counterFile, []byte("0\n"), 0o644); err != nil {
			return fmt.Errorf("ingest: reset counter: %w", err)
		}
	}

	// Auto-qmd-reindex (best-effort).
	qmdRC := 0
	if envFromConfig(r, "AWIKI_QMD_STATUS") != "missing" && qmdOnPATH() {
		if r.Qmd != nil {
			if _, code, err := r.Qmd.Reindex(ctx, r.RepoRoot); err != nil || code != 0 {
				qmdRC = 5
			}
		}
	}

	// Emit AGENT-PROMPT.
	prompt := fmt.Sprintf(agentPromptFormat, dest, mode)
	fmt.Fprintf(stdout, "AGENT-PROMPT|%s\n", prompt)

	// Optional agent invocation.
	agentRC := 0
	if opts.AgentCLI != "" {
		if mode != "batch" {
			fmt.Fprintf(stderr, "AGENT-SKIP|reason=mode-needs-human|mode=%s|cli=%s\n", mode, opts.AgentCLI)
		} else if !cliOnPATH(opts.AgentCLI) {
			fmt.Fprintf(stderr, "AGENT-SKIP|reason=cli-not-found|cli=%s\n", opts.AgentCLI)
			agentRC = 6
		} else {
			fmt.Fprintf(stdout, "AGENT-INVOKE|cli=%s|mode=%s\n", opts.AgentCLI, mode)
			if r.Agent != nil {
				if code, err := r.Agent.Run(ctx, opts.AgentCLI, opts.AgentFlags, prompt); err != nil || code != 0 {
					agentRC = 6
				}
			}
		}
	}

	switch {
	case lintRC != 0:
		return &ExitError{Code: 4, Msg: "auto-lint failed"}
	case qmdRC != 0:
		return &ExitError{Code: 5, Msg: "auto-qmd-reindex failed"}
	case agentRC != 0:
		return &ExitError{Code: 6, Msg: "agent failed"}
	}
	return nil
}

// envFromConfig returns the first non-empty value of key found in
// r.Config (loaded from .awiki/config) or the process environment.
// Mirrors the bash precedence: `.awiki/config` is sourced before the
// envvar test, so a config-set value wins if present.
func envFromConfig(r *Runner, key string) string {
	if v, ok := r.Config[key]; ok && v != "" {
		return v
	}
	return os.Getenv(key)
}

// LookPathFn is a swappable hook for tests to override exec.LookPath.
// Production wires it to exec.LookPath in init(); tests reassign it to
// a deterministic stub so they need not mutate $PATH.
var LookPathFn = exec.LookPath

// qmdOnPATH reports whether the `qmd` binary resolves on the current
// process PATH. Mirrors `command -v qmd >/dev/null 2>&1` at
// scripts/ingest.sh:102.
func qmdOnPATH() bool {
	return cliOnPATH("qmd")
}

// cliOnPATH reports whether the given CLI resolves on PATH. Mirrors the
// bash `command -v "$AGENT_CLI" >/dev/null 2>&1` test at
// scripts/ingest.sh:122.
func cliOnPATH(cli string) bool {
	if cli == "" {
		return false
	}
	_, err := LookPathFn(cli)
	return err == nil
}
