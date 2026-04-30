package synth

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// MindmapValidateExit reports the exit code class of a mindmap
// validation. Mirrors the bash port's `exit <n>` discipline so the
// caller (synthesis post-hook) can distinguish the failure modes.
const (
	MindmapValidateOK              = 0 // pass
	MindmapValidateUsage           = 1 // bad args / missing arg
	MindmapValidateMissingPage     = 2 // page file not found
	MindmapValidateNoMermaidBlock  = 3 // no mermaid fence inside generated region
	MindmapValidateMmdcRejected    = 4 // mermaid-cli rejected the block
	MindmapValidateBadFirstLine    = 5 // mermaid block does not start with `mindmap`
	MindmapValidateBraceImbalance  = 6 // shape brackets ()[]{} are imbalanced
)

// MindmapValidateOptions controls a single validation invocation.
type MindmapValidateOptions struct {
	// Page is the absolute or relative path to the synthesis page.
	Page string
	// RepoRoot is the wiki root. When non-empty the validator writes
	// a `.awiki/post-hook-ran` sentinel (mirrors the bash port; the
	// plugin-gate test depends on this side effect).
	RepoRoot string
	// MmdcLookup overrides exec.LookPath("mmdc"). Tests use it to
	// pin the "mmdc absent" branch deterministically.
	MmdcLookup func(name string) (string, error)
	// MmdcRunner overrides the real exec call to mmdc. The function
	// receives the path of a temp file holding the mermaid block and
	// must return nil on success, non-nil on rejection.
	MmdcRunner func(ctx context.Context, mmdcPath, blockTmp string) error
}

// MindmapValidate runs the mindmap post-hook validation on a single
// synthesis page. Returns the exit-code class and a human-readable
// error written to stderr by the caller (or nil on success).
//
// The Go port matches the bash script byte-for-byte on the
// happy-path and on every documented failure mode. Side effect:
// when RepoRoot is non-empty the function `touch`es
// <RepoRoot>/.awiki/post-hook-ran — the gate test in the synth
// plugin suite asserts this sentinel.
func MindmapValidate(ctx context.Context, opts MindmapValidateOptions, stderr io.Writer) int {
	if opts.Page == "" {
		fmt.Fprintln(stderr, "usage: synth-mindmap-validate <page>")
		return MindmapValidateUsage
	}
	info, err := os.Stat(opts.Page)
	if err != nil || info.IsDir() {
		fmt.Fprintf(stderr, "ERROR: page not found: %s\n", opts.Page)
		return MindmapValidateMissingPage
	}

	if opts.RepoRoot != "" {
		if err := touchPostHookSentinel(opts.RepoRoot); err != nil {
			// Sentinel failure is not fatal — the bash version uses
			// `mkdir -p` + `: > file` which will succeed in normal
			// trees. Surface a hint and keep going.
			fmt.Fprintf(stderr, "WARN: could not write post-hook sentinel: %v\n", err)
		}
	}

	block, err := extractMindmapBlock(opts.Page)
	if err != nil {
		fmt.Fprintf(stderr, "ERROR: %v\n", err)
		return MindmapValidateNoMermaidBlock
	}
	if strings.TrimSpace(block) == "" {
		fmt.Fprintf(stderr, "ERROR: no mermaid block found in generated region of %s\n", opts.Page)
		return MindmapValidateNoMermaidBlock
	}

	// 1) prefer mmdc dry-run if installed
	mmdcLookup := opts.MmdcLookup
	if mmdcLookup == nil {
		mmdcLookup = exec.LookPath
	}
	if mmdcPath, lookErr := mmdcLookup("mmdc"); lookErr == nil && mmdcPath != "" {
		tmp, err := os.CreateTemp("", "mindmap-*.mmd")
		if err != nil {
			fmt.Fprintf(stderr, "ERROR: tempfile: %v\n", err)
			return MindmapValidateUsage
		}
		tmpPath := tmp.Name()
		_, _ = tmp.WriteString(block)
		_ = tmp.Close()
		defer os.Remove(tmpPath)

		runner := opts.MmdcRunner
		if runner == nil {
			runner = defaultMmdcRunner
		}
		if err := runner(ctx, mmdcPath, tmpPath); err != nil {
			fmt.Fprintf(stderr, "ERROR: mmdc rejected mermaid block in %s\n", opts.Page)
			return MindmapValidateMmdcRejected
		}
		return MindmapValidateOK
	}

	// 2) regex sanity fallback
	firstLine := firstNonEmptyLine(block)
	if !strings.HasPrefix(firstLine, "mindmap") {
		fmt.Fprintf(stderr, "ERROR: mermaid block must start with 'mindmap' (got: %s)\n", firstLine)
		return MindmapValidateBadFirstLine
	}
	openCount, closeCount := countShapeBrackets(block)
	if openCount != closeCount {
		fmt.Fprintf(stderr, "ERROR: unbalanced shape brackets in mermaid block (%d open, %d close)\n", openCount, closeCount)
		return MindmapValidateBraceImbalance
	}
	return MindmapValidateOK
}

// extractMindmapBlock walks the page and returns the body of the
// first ```mermaid``` fence inside the BEGIN/END GENERATED region,
// matching the awk pipeline in the bash port. Lines outside the
// region (and lines outside the fence) are ignored.
func extractMindmapBlock(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var (
		inRegion bool
		inFence  bool
		out      strings.Builder
	)
	for scanner.Scan() {
		line := scanner.Text()
		if !inRegion {
			if strings.HasPrefix(line, "<!-- BEGIN GENERATED ") && strings.HasSuffix(line, " -->") {
				inRegion = true
			}
			continue
		}
		// inRegion
		if line == "<!-- END GENERATED -->" {
			inRegion = false
			inFence = false
			continue
		}
		if !inFence {
			if line == "```mermaid" {
				inFence = true
			}
			continue
		}
		// inFence
		if line == "```" {
			inFence = false
			continue
		}
		out.WriteString(line)
		out.WriteString("\n")
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return out.String(), nil
}

// firstNonEmptyLine returns the first non-blank line of s with no
// trailing newline. Returns "" when s contains only blanks.
func firstNonEmptyLine(s string) string {
	for line := range strings.SplitSeq(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			return t
		}
	}
	return ""
}

// countShapeBrackets matches the bash `tr -dc '([{' | wc -c` and the
// closing-bracket equivalent. Bash counts characters, not balanced
// pairs — `(]` would still pass the check. We replicate the loose
// semantics so existing pages pass/fail the same way.
func countShapeBrackets(s string) (open, close int) {
	for _, r := range s {
		switch r {
		case '(', '[', '{':
			open++
		case ')', ']', '}':
			close++
		}
	}
	return open, close
}

// touchPostHookSentinel writes (or truncates) <repoRoot>/.awiki/post-hook-ran.
// The bash port emitted this sentinel for the synth plugin gate test.
func touchPostHookSentinel(repoRoot string) error {
	if repoRoot == "" {
		return errors.New("empty repoRoot")
	}
	dir := filepath.Join(repoRoot, ".awiki")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(dir, "post-hook-ran"))
	if err != nil {
		return err
	}
	return f.Close()
}

// defaultMmdcRunner wraps `mmdc -i <block> -o /dev/null --quiet` to
// match the bash port. Stdout/stderr are discarded — the bash port
// pipes them to /dev/null too.
func defaultMmdcRunner(ctx context.Context, mmdcPath, blockTmp string) error {
	cmd := exec.CommandContext(ctx, mmdcPath, "-i", blockTmp, "-o", "/dev/null", "--quiet")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}
