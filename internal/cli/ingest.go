package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"awiki/internal/adapters"
	"awiki/internal/config"
	"awiki/internal/ingest"
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
		RepoRoot:   repoRoot,
		ContentDir: filepath.Join(repoRoot, "content"),
		Config:     cfg,
		PDFToText:  adapters.ExecPDFToText{},
		Whisper:    adapters.ExecWhisper{},
		XLSX2CSV:   adapters.ExecXLSX2CSV{},
		FSNotify:   adapters.ExecFSNotify{},
		GitExt:     adapters.ExecGitExt{},
		Qmd:        adapters.ExecQmd{},
		Today:      todayDate(),
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

func runIngestPDF(_ *ingest.Runner, _ []string, _, stderr io.Writer) int {
	return notYetPorted("ingest-pdf", stderr)
}

func runIngestAudio(_ *ingest.Runner, _ []string, _, stderr io.Writer) int {
	return notYetPorted("ingest-audio", stderr)
}

func runCapture(_ *ingest.Runner, _ []string, _, stderr io.Writer) int {
	return notYetPorted("capture", stderr)
}

func runWatchdog(_ *ingest.Runner, _ []string, _, stderr io.Writer) int {
	return notYetPorted("watchdog", stderr)
}

func runIngestBatchList(_ *ingest.Runner, _ []string, _, stderr io.Writer) int {
	return notYetPorted("ingest-batch-list", stderr)
}

func runIngestGitList(_ *ingest.Runner, _ []string, _, stderr io.Writer) int {
	return notYetPorted("ingest-git-list", stderr)
}
