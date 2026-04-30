package ops

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// DeleteOptions configures a `awiki delete <slug>` invocation. Mirrors
// scripts/delete-page.sh.
type DeleteOptions struct {
	Slug       string
	RepoRoot   string
	ContentDir string
	GitRm      GitRemover
	Logger     LogAppender
}

// GitRemover wraps `git rm <path>` with a fallback to plain remove,
// mirroring the bash form `git rm "$PAGE" 2>/dev/null || rm "$PAGE"`.
type GitRemover interface {
	Remove(ctx context.Context, repoRoot, path string) error
}

// Delete removes the page and replaces wikilinks with broken markers.
// Returns:
//
//	0  success
//	2  slug not found
func Delete(opts DeleteOptions, stdout, stderr io.Writer) int {
	if opts.Slug == "" {
		fmt.Fprintln(stderr, "usage: delete-page.sh <slug>")
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
	rm := opts.GitRm
	if rm == nil {
		rm = execGitRemover{}
	}
	logger := opts.Logger
	if logger == nil {
		logger = bashLogAppender{}
	}

	page, err := findFirstSlugFile(contentDir, opts.Slug)
	if err != nil || page == "" {
		fmt.Fprintf(stderr, "slug not found: %s\n", opts.Slug)
		return 2
	}

	if err := rm.Remove(context.Background(), repoRoot, page); err != nil {
		// Fallback: plain os.Remove.
		if err2 := os.Remove(page); err2 != nil {
			fmt.Fprintf(stderr, "delete: %v\n", err2)
			return 1
		}
	}

	if err := markWikilinksBroken(contentDir, opts.Slug); err != nil {
		fmt.Fprintf(stderr, "delete: rewrite: %v\n", err)
		return 1
	}

	_ = logger.Append(context.Background(), repoRoot, "delete",
		fmt.Sprintf("%s (removed; wikilinks marked broken for review)", opts.Slug))

	fmt.Fprintf(stdout, "DELETE-OK|slug=%s\n", opts.Slug)
	return 0
}

// DeleteCLI parses argv and dispatches.
func DeleteCLI(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: delete-page.sh <slug>")
		return 1
	}
	return Delete(DeleteOptions{Slug: args[0]}, stdout, stderr)
}

// markWikilinksBroken replaces wikilinks pointing at the deleted slug
// with the broken-marker form used by scripts/delete-page.sh:
//
//	gsub("\\[\\[" slug "\\]\\]", "<!-- broken: was [[" slug "]] -->" slug)
//	gsub("\\[\\[" slug "\\|",    "<!-- broken: was [[" slug "|... -->" slug "|")
func markWikilinksBroken(contentDir, slug string) error {
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
		full := "[[" + slug + "]]"
		fullRepl := "<!-- broken: was [[" + slug + "]] -->" + slug
		s = strings.ReplaceAll(s, full, fullRepl)
		piped := "[[" + slug + "|"
		pipedRepl := "<!-- broken: was [[" + slug + "|... -->" + slug + "|"
		s = strings.ReplaceAll(s, piped, pipedRepl)
		if string(data) == s {
			return nil
		}
		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, []byte(s), 0o644); err != nil {
			return err
		}
		return os.Rename(tmp, path)
	})
}

type execGitRemover struct{}

func (execGitRemover) Remove(ctx context.Context, repoRoot, path string) error {
	cmd := newCmd(ctx, "git", "-C", repoRoot, "rm", path)
	return drainCmd(cmd)
}
