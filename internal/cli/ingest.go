package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"awiki/internal/adapters"
	"awiki/internal/config"
	"awiki/internal/ingest"
	"awiki/internal/ingest/formats"
)

// runIngestVerb dispatches the flat ingest verbs (`awiki ingest`,
// `awiki ingest-xlsx`, etc.). Each verb maps to a single sub-runner;
// during the infra slice every sub-runner returns the "not yet ported"
// sentinel so subsequent slices can wire one verb at a time without
// touching the dispatcher.
func runIngestVerb(verb string, args []string, stdout, stderr io.Writer) int {
	r, err := buildIngestRunner()
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", verb, err)
		return 1
	}
	switch verb {
	case "ingest":
		return runIngest(r, args, stdout, stderr)
	case "ingest-xlsx":
		return runIngestXLSX(r, args, stdout, stderr)
	case "ingest-git":
		return runIngestGit(r, args, stdout, stderr)
	case "ingest-pdf":
		return runIngestPDF(r, args, stdout, stderr)
	case "ingest-audio":
		return runIngestAudio(r, args, stdout, stderr)
	case "capture":
		return runCapture(r, args, stdout, stderr)
	case "watchdog":
		return runWatchdog(r, args, stdout, stderr)
	case "ingest-batch-list":
		return runIngestBatchList(r, args, stdout, stderr)
	case "ingest-git-list":
		return runIngestGitList(r, args, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "ingest: unknown verb %q\n", verb)
		return 1
	}
}

func buildIngestRunner() (*ingest.Runner, error) {
	repoRoot := os.Getenv("AWIKI_REPO_ROOT")
	if repoRoot == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		repoRoot = wd
	}
	if abs, err := filepath.Abs(repoRoot); err == nil {
		repoRoot = abs
	}
	cfg, _ := config.Load(filepath.Join(repoRoot, ".awiki", "config"))
	return &ingest.Runner{
		RepoRoot:    repoRoot,
		ContentDir:  filepath.Join(repoRoot, "content"),
		Config:      cfg,
		PDFToText:   adapters.ExecPDFToText{},
		Whisper:     adapters.ExecWhisper{},
		XLSXExtract: adapters.ExecXLSXExtract{RepoRoot: repoRoot},
		FSNotify:    adapters.ExecFSNotify{},
		GitExt:      adapters.ExecGitExt{},
		Qmd:         adapters.ExecQmd{},
		Today:       todayDate(),
	}, nil
}

// notYetPorted emits the standard sentinel error that each verb sub-runner
// returns until its slice lands.
func notYetPorted(verb string, stderr io.Writer) int {
	fmt.Fprintf(stderr, "ingest: verb not yet ported: %s\n", verb)
	return 1
}

func runIngest(_ *ingest.Runner, _ []string, _, stderr io.Writer) int {
	return notYetPorted("ingest", stderr)
}

func runIngestXLSX(_ *ingest.Runner, _ []string, _, stderr io.Writer) int {
	return notYetPorted("ingest-xlsx", stderr)
}

func runIngestGit(_ *ingest.Runner, _ []string, _, stderr io.Writer) int {
	return notYetPorted("ingest-git", stderr)
}

// runIngestPDF parses `awiki ingest-pdf <path>` and delegates to
// formats.IngestPDF. Mirrors the bash usage at scripts/ingest-pdf.sh:4
// — exactly one positional arg; --help prints usage and exits 0.
func runIngestPDF(r *ingest.Runner, args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(stdout, "usage: awiki ingest-pdf <pdf-path-under-raw/inbox/>")
		return 0
	}
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: ingest-pdf.sh <pdf-path-under-raw/inbox/>")
		return 1
	}
	_, err := formats.IngestPDF(context.Background(), r, formats.PDFOptions{SourcePath: args[0]}, stdout, stderr)
	if err != nil {
		var ee *ingest.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode()
		}
		fmt.Fprintf(stderr, "ingest-pdf: %v\n", err)
		return 1
	}
	return 0
}

// runIngestAudio parses `awiki ingest-audio <path>` and delegates to
// formats.IngestAudio. Mirrors bash usage at scripts/ingest-audio.sh:4.
func runIngestAudio(r *ingest.Runner, args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(stdout, "usage: awiki ingest-audio <audio-path-under-raw/inbox/>")
		return 0
	}
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: ingest-audio.sh <audio-path-under-raw/inbox/>")
		return 1
	}
	_, err := formats.IngestAudio(context.Background(), r, formats.AudioOptions{SourcePath: args[0]}, stdout, stderr)
	if err != nil {
		var ee *ingest.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode()
		}
		fmt.Fprintf(stderr, "ingest-audio: %v\n", err)
		return 1
	}
	return 0
}

// runCapture parses the `awiki capture -- <text...>` argument shape
// (mirrors scripts/capture.sh) and delegates to ingest.Runner.Capture.
// Argument grammar:
//   - args[0] == "--help" / "-h" -> print usage on stdout, exit 0.
//   - first arg must be "--"; remaining args are joined with spaces.
//   - missing "--" -> exit 1 with the bash-compat error.
func runCapture(r *ingest.Runner, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		captureUsage(stderr)
		return 1
	}
	if args[0] == "--help" || args[0] == "-h" {
		captureUsage(stdout)
		return 0
	}
	if args[0] != "--" {
		fmt.Fprintln(stderr, "ERROR|missing '--' separator (use: capture.sh -- \"<text>\")")
		return 1
	}
	rest := args[1:]
	if len(rest) == 0 {
		fmt.Fprintln(stderr, "ERROR|empty: no text after '--'")
		return 4
	}
	opts := ingest.CaptureOptions{
		Text:         strings.Join(rest, " "),
		Presanitized: os.Getenv("AWIKI_CAPTURE_PRESANITIZED") == "1",
		InboxPath:    os.Getenv("AWIKI_INBOX_FILE"),
	}
	if err := r.Capture(opts, stdout, stderr); err != nil {
		var ee *ingest.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode()
		}
		fmt.Fprintf(stderr, "capture: %v\n", err)
		return 1
	}
	return 0
}

func captureUsage(w io.Writer) {
	fmt.Fprintln(w, `usage: awiki capture -- "<text>"`)
	fmt.Fprintln(w, "  Appends a sanitized capture line to content/inbox.md.")
	fmt.Fprintln(w, "  Set AWIKI_INBOX_FILE to override the destination.")
	fmt.Fprintln(w, "  Set AWIKI_CAPTURE_PRESANITIZED=1 if upstream already sanitized.")
}

func runWatchdog(_ *ingest.Runner, _ []string, _, stderr io.Writer) int {
	return notYetPorted("watchdog", stderr)
}

// runIngestBatchList delegates to ingest.Runner.ListBatch. Mirrors the
// inline justfile recipe `find raw/inbox/batch -type f | sort`. Accepts
// no flags or args; emits a usage hint on stderr if any are supplied.
func runIngestBatchList(r *ingest.Runner, args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintln(stderr, "usage: awiki ingest-batch-list")
		return 1
	}
	if err := r.ListBatch(stdout); err != nil {
		fmt.Fprintf(stderr, "ingest-batch-list: %v\n", err)
		return 1
	}
	return 0
}

// runIngestGitList delegates to ingest.Runner.ListGit. Mirrors the inline
// justfile recipe `ls -1 .awiki/git-state/ 2>/dev/null | sed 's/\.json$//'`.
func runIngestGitList(r *ingest.Runner, args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintln(stderr, "usage: awiki ingest-git-list")
		return 1
	}
	if err := r.ListGit(stdout); err != nil {
		fmt.Fprintf(stderr, "ingest-git-list: %v\n", err)
		return 1
	}
	return 0
}
