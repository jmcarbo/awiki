package synth

import (
	"context"
	"strings"
)

// HandEditCheck reports whether the BEGIN..END region of the live
// page has diverged from its committed (HEAD) version. Returns
// (handEdited, untracked, err).
//
// Matches scripts/synth.sh:synth_handedit_check semantics:
//   - untracked file → untracked=true (caller treats as fresh)
//   - no HEAD yet → untracked=true
//   - regions equal → handEdited=false
//   - regions differ → handEdited=true
func (r *Runner) HandEditCheck(ctx context.Context, pagePath, workingText string) (handEdited bool, untracked bool, err error) {
	tracked, err := r.Git.LsFiles(ctx, r.RepoRoot, pagePath)
	if err != nil {
		return false, false, err
	}
	if !tracked {
		return false, true, nil
	}
	head, ok, err := r.Git.ShowHead(ctx, r.RepoRoot, pagePath)
	if err != nil {
		return false, false, err
	}
	if !ok {
		return false, true, nil
	}
	return extractRegion(head) != extractRegion(workingText), false, nil
}

// extractRegion returns lines from BEGIN GENERATED through END
// GENERATED inclusive (both as substrings). Mirrors the bash awk
// extractor.
func extractRegion(text string) string {
	beginIdx := strings.Index(text, "<!-- BEGIN GENERATED ")
	endIdx := strings.Index(text, "<!-- END GENERATED -->")
	if beginIdx < 0 || endIdx < 0 || beginIdx > endIdx {
		return ""
	}
	endLineEnd := endIdx + len("<!-- END GENERATED -->")
	if endLineEnd < len(text) && text[endLineEnd] == '\n' {
		endLineEnd++
	}
	return text[beginIdx:endLineEnd]
}
