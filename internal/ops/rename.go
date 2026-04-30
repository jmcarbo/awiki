package ops

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// RenameOptions configures a rename operation. Mirrors
// scripts/rename.sh: find content/<old>.md, git-mv to <new>.md, rewrite
// wikilinks across content/**/*.md.
type RenameOptions struct {
	OldSlug    string
	NewSlug    string
	RepoRoot   string         // default cwd
	ContentDir string         // default <repoRoot>/content
	GitMover   GitMover       // default execGitMover
	Logger     LogAppender    // default file-based; injected for tests
}

// GitMover wraps `git mv <src> <dst>` with a fallback to plain rename.
// The bash form: `git mv "$OLD_PATH" "$NEW_PATH" 2>/dev/null || mv "$OLD_PATH" "$NEW_PATH"`.
type GitMover interface {
	Move(ctx context.Context, repoRoot, src, dst string) error
}

// LogAppender is the journal sink for rename/delete events. The
// adapter implementation calls scripts/log-append.sh; the in-Go
// implementation calls ops.Log.
type LogAppender interface {
	Append(ctx context.Context, repoRoot, action, msg string) error
}

// Rename moves the page and rewrites wikilinks. Returns:
//   - exit code per bash contract:
//     0  success
//     2  slug not found
//     3  target already exists
func Rename(opts RenameOptions, stdout, stderr io.Writer) int {
	if opts.OldSlug == "" || opts.NewSlug == "" {
		fmt.Fprintln(stderr, "usage: rename <old-slug> <new-slug>")
		return 1
	}
	repoRoot := opts.RepoRoot
	if repoRoot == "" {
		repoRoot, _ = os.Getwd()
	}
	contentDir := opts.ContentDir
	if contentDir == "" {
		contentDir = filepath.Join(repoRoot, "content")
	}
	mover := opts.GitMover
	if mover == nil {
		mover = execGitMover{}
	}
	logger := opts.Logger
	if logger == nil {
		logger = bashLogAppender{}
	}

	// 1. Find first content/**/<old>.md (mirrors `find content -name "$OLD.md"`).
	oldPath, err := findFirstSlugFile(contentDir, opts.OldSlug)
	if err != nil || oldPath == "" {
		fmt.Fprintf(stderr, "slug not found: %s\n", opts.OldSlug)
		return 2
	}

	newPath := filepath.Join(filepath.Dir(oldPath), opts.NewSlug+".md")
	if _, err := os.Stat(newPath); err == nil {
		fmt.Fprintf(stderr, "target exists: %s\n", newPath)
		return 3
	}

	// 2. git mv (with plain rename fallback).
	if err := mover.Move(context.Background(), repoRoot, oldPath, newPath); err != nil {
		// Fall back to plain rename. Bash: `git mv ... 2>/dev/null || mv ...`.
		if err2 := os.Rename(oldPath, newPath); err2 != nil {
			fmt.Fprintf(stderr, "rename: %v\n", err2)
			return 1
		}
	}

	// 3. Rewrite wikilinks across content/**/*.md.
	if err := rewriteWikilinks(contentDir, opts.OldSlug, opts.NewSlug); err != nil {
		fmt.Fprintf(stderr, "rename: rewrite: %v\n", err)
		return 1
	}

	// 4. Log the rename.
	_ = logger.Append(context.Background(), repoRoot, "rename",
		fmt.Sprintf("%s -> %s", opts.OldSlug, opts.NewSlug))

	fmt.Fprintf(stdout, "RENAME-OK|old=%s|new=%s\n", opts.OldSlug, opts.NewSlug)
	return 0
}

// RenameCLI parses argv and dispatches.
func RenameCLI(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		fmt.Fprintln(stderr, "usage: rename.sh <old-slug> <new-slug>")
		return 1
	}
	return Rename(RenameOptions{OldSlug: args[0], NewSlug: args[1]}, stdout, stderr)
}

// findFirstSlugFile walks contentDir and returns the first <slug>.md
// (mirroring the bash `find content -name "$SLUG.md" -type f | head -1`).
// Walk order is deterministic alphabetical via filepath.WalkDir.
func findFirstSlugFile(contentDir, slug string) (string, error) {
	want := slug + ".md"
	var hit string
	err := filepath.WalkDir(contentDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() == want && hit == "" {
			hit = path
		}
		return nil
	})
	return hit, err
}

// rewriteWikilinks performs an awk-equivalent in-place edit on every
// .md under contentDir. The bash awk form:
//
//	gsub("\\[\\[" old "\\]\\]", "[[" new "]]")
//	gsub("\\[\\[" old "\\|", "[[" new "|")
//
// We mirror the literal string substitutions byte-for-byte.
func rewriteWikilinks(contentDir, oldSlug, newSlug string) error {
	return filepath.WalkDir(contentDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		s := string(data)
		s = strings.ReplaceAll(s, "[["+oldSlug+"]]", "[["+newSlug+"]]")
		s = strings.ReplaceAll(s, "[["+oldSlug+"|", "[["+newSlug+"|")
		if string(data) == s {
			return nil
		}
		// Bash uses `awk ... > "$f.tmp" && mv "$f.tmp" "$f"`.
		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, []byte(s), 0o644); err != nil {
			return err
		}
		return os.Rename(tmp, path)
	})
}

// --- adapters ---

type execGitMover struct{}

func (execGitMover) Move(ctx context.Context, repoRoot, src, dst string) error {
	return runGitMv(ctx, repoRoot, src, dst)
}

func runGitMv(ctx context.Context, repoRoot, src, dst string) error {
	// Use a thin Cmd here (not adapters/git_ext.go) because git-mv is
	// not in the GitExt interface and the fallback path is simpler.
	cmd := newCmd(ctx, "git", "-C", repoRoot, "mv", src, dst)
	return cmd.Run()
}

// bashLogAppender shells the existing log-append adapter (which now
// has a Go backend via ops.Log; see the production wiring in
// runRename for tests).
type bashLogAppender struct{}

func (bashLogAppender) Append(ctx context.Context, repoRoot, action, msg string) error {
	// Native Go log append (in-process). Mirrors the bash form's
	// `bash scripts/log-append.sh ...` semantics.
	logFile := os.Getenv("AWIKI_LOG_FILE")
	if logFile == "" {
		logFile = filepath.Join(repoRoot, "content", "log.md")
	}
	_, err := Log(LogOptions{Action: action, Message: msg, LogFile: logFile})
	return err
}

