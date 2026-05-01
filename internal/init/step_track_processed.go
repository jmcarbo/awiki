package initverb

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// stepTrackProcessedImpl runs Step 4: opt-in to tracking ingested
// sources by removing the two raw/processed lines from .gitignore.
//
// The bash oracle:
//
//	raw/processed/*
//	!raw/processed/.gitkeep
//
// stay in the file by default; on `y` we strip exactly those two
// lines (the _originals/ + private/ exceptions remain so privacy is
// preserved). Idempotent: re-runs that find the lines already gone
// report "already applied" without rewriting the file.
type stepTrackProcessed struct{}

func (stepTrackProcessed) ID() string          { return "track-processed" }
func (stepTrackProcessed) Description() string { return "track ingested sources?" }
func (stepTrackProcessed) Kind() Kind          { return KindHybrid }

func (stepTrackProcessed) Execute(ctx StepContext, ans *Answers) (Result, error) {
	if !ans.TrackProcessed && !ctx.NonInteractive {
		if ctx.Confirm == nil {
			return Result{Status: StatusFailed}, fmt.Errorf("track-processed: Confirm is nil")
		}
		ok, err := ctx.Confirm("Track ingested sources in git?", false)
		if err != nil {
			return Result{Status: StatusFailed}, err
		}
		ans.TrackProcessed = ok
	}
	if !ans.TrackProcessed {
		return Result{Status: StatusApplied, Note: "track_processed=false (default)"}, nil
	}
	gi := filepath.Join(ctx.RepoRoot, ".gitignore")
	body, err := os.ReadFile(gi)
	if err != nil {
		return Result{Status: StatusFailed}, fmt.Errorf("read .gitignore: %w", err)
	}
	const dropA = "raw/processed/*"
	const dropB = "!raw/processed/.gitkeep"
	var kept []string
	stripped := 0
	for _, line := range strings.Split(string(body), "\n") {
		trim := strings.TrimSpace(line)
		if trim == dropA || trim == dropB {
			stripped++
			continue
		}
		kept = append(kept, line)
	}
	if stripped == 0 {
		return Result{Status: StatusApplied, Note: "track_processed=true (already in .gitignore)"}, nil
	}
	out := strings.Join(kept, "\n")
	if err := os.WriteFile(gi, []byte(out), 0o644); err != nil {
		return Result{Status: StatusFailed}, fmt.Errorf("write .gitignore: %w", err)
	}
	fmt.Fprintln(ctx.Stdout, "WARN: tracking ingested sources may include copyrighted material; history-rewrite cost is non-trivial if revoked.")
	return Result{Status: StatusApplied, Note: fmt.Sprintf("removed %d lines from .gitignore", stripped)}, nil
}
