package git

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"awiki/internal/adapters"
)

// RunOptions carries the flag surface scripts/ingest-git.sh accepts
// (lines 17-29). Each field maps 1:1 to a `--<flag>=<value>`.
type RunOptions struct {
	Spec             string // <repo-spec> positional
	PathsOverride    string // --paths=<csv>
	Private          bool   // --private
	ProtectEdits     bool   // --protect-edits
	Summarize        bool   // --summarize (reserved)
	DryRun           bool   // --dry-run
	RepoNameOverride string // --repo-name=<name>
}

// Run is the top-level driver. Mirrors scripts/ingest-git.sh.
func Run(ctx context.Context, repoRoot string, gitExt adapters.GitExt, opts RunOptions, stdout, stderr io.Writer) error {
	if opts.Spec == "" {
		return &RunError{Code: 1, Msg: "usage: ingest-git.sh <repo-spec> [flags]"}
	}

	repoKey, err := RepoKey(repoRoot, opts.Spec)
	if err != nil {
		return &RunError{Code: 1, Msg: err.Error()}
	}
	repoName := opts.RepoNameOverride
	if repoName == "" {
		repoName = strings.TrimPrefix(repoKey, "local-")
	}
	private := opts.Private
	if IsSSH(opts.Spec) {
		private = true
	}

	// Resolve checkout (clone or fetch).
	resolution, err := ResolveCheckout(ctx, repoRoot, gitExt, opts.Spec, repoKey)
	if err != nil {
		// ResolveError carries bash-equivalent exit code (10/11).
		fmt.Fprintf(stderr, "ERROR: %v\n", err)
		return err
	}

	// Determine include paths.
	defaultPaths := "README.md,docs/,rfcs/,adr/"
	pathsCSV := opts.PathsOverride
	if pathsCSV == "" {
		pathsCSV = defaultPaths
	}
	includePaths := strings.Split(pathsCSV, ",")

	// List markdown files in the repo via `git ls-tree -r HEAD --name-only`.
	lsOut, _, err := gitExt.LsTree(ctx, resolution.Checkout, "HEAD")
	if err != nil {
		fmt.Fprintf(stderr, "ERROR: ls-tree failed: %v\n", err)
		return &RunError{Code: 11, Msg: "ls-tree failed"}
	}
	allFiles := strings.Split(strings.TrimRight(lsOut, "\n"), "\n")
	currentFiles := []string{}
	for _, rel := range allFiles {
		if rel == "" {
			continue
		}
		if !isMarkdown(rel) {
			continue
		}
		if !filterPaths(rel, includePaths) {
			continue
		}
		currentFiles = append(currentFiles, rel)
	}
	sort.Strings(currentFiles)

	// Build current blob map.
	currentBlob := make(map[string]string, len(currentFiles))
	for _, rel := range currentFiles {
		blob, _, _ := gitExt.RevParse(ctx, resolution.Checkout, "HEAD:"+rel)
		currentBlob[rel] = blob
	}

	// Load prior state.
	priorState, _, err := LoadState(repoRoot, repoKey)
	if err != nil {
		fmt.Fprintf(stderr, "ERROR: load state: %v\n", err)
		return &RunError{Code: 1, Msg: "state load"}
	}
	priorBlob := map[string]string{}
	priorSlug := map[string]string{}
	for rel, info := range priorState.Files {
		priorBlob[rel] = info.BlobSHA
		priorSlug[rel] = info.Slug
	}

	// Diff.
	added := []string{}
	modified := []string{}
	unchanged := []string{}
	removed := []string{}
	currentSet := map[string]bool{}
	for _, rel := range currentFiles {
		currentSet[rel] = true
		pblob := priorBlob[rel]
		cblob := currentBlob[rel]
		switch {
		case pblob == "":
			added = append(added, rel)
		case pblob != cblob:
			modified = append(modified, rel)
		default:
			unchanged = append(unchanged, rel)
		}
	}
	for prel := range priorBlob {
		if !currentSet[prel] {
			removed = append(removed, prel)
		}
	}
	sort.Strings(removed)

	// Slug map + collision detection.
	slugMap := make(map[string]string, len(currentFiles))
	for _, rel := range currentFiles {
		slugMap[rel] = flattenSlug(repoName, rel)
	}
	seen := map[string]string{}
	for _, rel := range currentFiles {
		s := slugMap[rel]
		if other, ok := seen[s]; ok {
			fmt.Fprintf(stderr, "ERROR: slug collision: %s ← %s and %s\n", s, other, rel)
			return &RunError{Code: 14, Msg: "slug collision"}
		}
		seen[s] = rel
	}

	// Output paths.
	outSources := "content/sources"
	outEntities := "content/entities"
	assetOutDir := "content/sources/_assets/git-" + repoName
	if private {
		outSources = "content/private/sources"
		outEntities = "content/private/entities"
		assetOutDir = "content/private/sources/_assets/git-" + repoName
	}
	for _, d := range []string{outSources, outEntities, assetOutDir} {
		if err := os.MkdirAll(filepath.Join(repoRoot, d), 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}

	entityPath := filepath.Join(repoRoot, outEntities, "repo-"+repoName+".md")
	if existing, err := os.ReadFile(entityPath); err == nil {
		existingURL := extractFrontmatterField(string(existing), "git_url")
		if existingURL != "" && existingURL != opts.Spec && opts.RepoNameOverride == "" {
			fmt.Fprintf(stderr, "ERROR: repo entity %s already exists with git_url=%s; pass --repo-name=<override>\n",
				entityPath, existingURL)
			return &RunError{Code: 15, Msg: "entity-exists-different-url"}
		}
	}

	// Dry-run: emit PLAN records and exit.
	privateFlag := "0"
	if private {
		privateFlag = "1"
	}
	if opts.DryRun {
		fmt.Fprintf(stdout, "PLAN|spec=%s|repo_key=%s|repo_name=%s|checkout=%s|head=%s|branch=%s|private=%s\n",
			opts.Spec, repoKey, repoName, resolution.Checkout, resolution.HeadSHA, resolution.DefaultBranch, privateFlag)
		fmt.Fprintf(stdout, "PLAN|added=%d|modified=%d|removed=%d|unchanged=%d\n",
			len(added), len(modified), len(removed), len(unchanged))
		for _, f := range added {
			fmt.Fprintf(stdout, "PLAN|add|%s\n", f)
		}
		for _, f := range modified {
			fmt.Fprintf(stdout, "PLAN|mod|%s\n", f)
		}
		for _, f := range removed {
			fmt.Fprintf(stdout, "PLAN|rem|%s\n", f)
		}
		return nil
	}

	// Transform added + modified files.
	toWrite := append([]string{}, added...)
	toWrite = append(toWrite, modified...)

	writeOK := 0
	writeFail := 0
	for _, rel := range toWrite {
		slug := slugMap[rel]
		out := filepath.Join(repoRoot, outSources, slug+".md")

		// --protect-edits: redirect to staging if existing page diverged.
		if opts.ProtectEdits {
			if data, err := os.ReadFile(out); err == nil {
				lastUpdated := extractFrontmatterField(string(data), "last_updated")
				lastIngested := ""
				if priorEntry, ok := priorState.Files[rel]; ok {
					lastIngested = priorEntry.LastIngested
				}
				lastIngestedDate := ""
				if len(lastIngested) >= 10 {
					lastIngestedDate = lastIngested[:10]
				}
				if lastUpdated != "" && lastIngestedDate != "" && lastUpdated > lastIngestedDate {
					stageDir := filepath.Join(repoRoot, "raw", "inbox", "checkpoint", ".staged")
					if err := os.MkdirAll(stageDir, 0o755); err != nil {
						fmt.Fprintf(stderr, "FAIL|protect-mkdir|%v\n", err)
					}
					proposalTarget := filepath.Join("raw", "inbox", "checkpoint", ".staged", slug+".md")
					out = filepath.Join(repoRoot, proposalTarget)
					fmt.Fprintf(stdout, "PROTECT|%s|→|%s\n", rel, proposalTarget)
				}
			}
		}

		txOpts := TransformOptions{
			InPath:       filepath.Join(resolution.Checkout, rel),
			OutPath:      out,
			RepoKey:      repoKey,
			RepoName:     repoName,
			RepoRelpath:  rel,
			GitURL:       opts.Spec,
			GitBlobSHA:   currentBlob[rel],
			AssetOutDir:  filepath.Join(repoRoot, assetOutDir),
			UpstreamRoot: resolution.Checkout,
			Private:      private,
			SlugMap:      slugMap,
		}
		if _, err := Transform(txOpts, stdout); err != nil {
			writeFail++
			fmt.Fprintf(stderr, "FAIL|transform|%s\n", rel)
		} else {
			writeOK++
		}
	}

	// Move derived pages for removed upstream files to graveyard.
	if len(removed) > 0 {
		graveyard := filepath.Join(repoRoot, "raw", "_originals", "git", repoKey)
		if err := os.MkdirAll(graveyard, 0o755); err != nil {
			fmt.Fprintf(stderr, "WARN|graveyard mkdir: %v\n", err)
		} else {
			for _, prel := range removed {
				slug := priorSlug[prel]
				if slug == "" {
					continue
				}
				src := filepath.Join(repoRoot, outSources, slug+".md")
				if _, err := os.Stat(src); err != nil {
					continue
				}
				dst := filepath.Join(graveyard, slug+".md")
				if err := os.Rename(src, dst); err == nil {
					fmt.Fprintf(stdout, "REMOVED|%s|→|%s\n", prel,
						filepath.ToSlash(filepath.Join("raw", "_originals", "git", repoKey, slug+".md")))
				}
			}
		}
	}

	// Persist state.
	nowISO := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	files := make(map[string]FileEntry, len(currentFiles))
	for _, rel := range currentFiles {
		slug := slugMap[rel]
		files[rel] = FileEntry{
			BlobSHA:      currentBlob[rel],
			Slug:         slug,
			OutPath:      filepath.ToSlash(filepath.Join(outSources, slug+".md")),
			LastIngested: nowISO,
		}
	}
	state := State{
		Schema:        1,
		RepoKey:       repoKey,
		RepoName:      repoName,
		URL:           opts.Spec,
		DefaultBranch: resolution.DefaultBranch,
		HeadSHA:       resolution.HeadSHA,
		IngestedAt:    nowISO,
		Private:       private,
		Files:         files,
	}
	if err := SaveState(repoRoot, repoKey, state); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	// Write the entity page.
	today := nowISO[:10]
	if err := writeEntityPage(entityPath, repoName, opts.Spec, resolution.DefaultBranch, resolution.HeadSHA, nowISO, today, private, currentFiles, slugMap); err != nil {
		return fmt.Errorf("write entity: %w", err)
	}

	fmt.Fprintf(stdout, "OK|repo_key=%s|added=%d|modified=%d|removed=%d|written=%d|failed=%d\n",
		repoKey, len(added), len(modified), len(removed), writeOK, writeFail)

	if writeFail > 0 {
		return &RunError{Code: 2, Msg: fmt.Sprintf("%d transform failures", writeFail)}
	}
	return nil
}

// flattenSlug mirrors scripts/ingest-git.sh:160-167 — strip the .md/.mdx
// suffix, lowercase, replace `/` with `-`, prepend `git-<repo>-`.
func flattenSlug(repoName, rel string) string {
	stem := rel
	if strings.HasSuffix(stem, ".mdx") {
		stem = stem[:len(stem)-4]
	} else if strings.HasSuffix(stem, ".md") {
		stem = stem[:len(stem)-3]
	}
	stem = strings.ToLower(stem)
	stem = strings.ReplaceAll(stem, "/", "-")
	return "git-" + repoName + "-" + stem
}

// filterPaths mirrors scripts/ingest-git.sh:89-104 — accept rel if it
// matches an entry, with `/` suffix indicating "directory prefix" and
// no suffix indicating "exact match". Reject node_modules / vendor /
// .git anywhere in the path.
func filterPaths(rel string, entries []string) bool {
	excluded := []string{"node_modules/", "vendor/", ".git/"}
	for _, ex := range excluded {
		if strings.HasPrefix(rel, ex) || strings.Contains(rel, "/"+ex) {
			return false
		}
	}
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.HasSuffix(entry, "/") {
			if strings.HasPrefix(rel, entry) {
				return true
			}
		} else if rel == entry {
			return true
		}
	}
	return false
}

// isMarkdown returns true for .md / .mdx names. Mirrors the bash regex
// `\.(md|mdx)$` at scripts/ingest-git.sh:87.
func isMarkdown(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".mdx")
}

// extractFrontmatterField pulls the value of a single key from a YAML
// frontmatter block. Mirrors `awk -F': ' '/^key: /{print $2; exit}'`.
func extractFrontmatterField(text, key string) string {
	prefix := key + ": "
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

// writeEntityPage writes the per-repo entity page. Mirrors
// scripts/ingest-git.sh:334-360.
func writeEntityPage(path, repoName, spec, branch, sha, nowISO, today string, private bool, currentFiles []string, slugMap map[string]string) error {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("title: \"%s\"\n", repoName))
	b.WriteString(fmt.Sprintf("date: %s\n", today))
	b.WriteString(fmt.Sprintf("last_updated: %s\n", today))
	b.WriteString("type: entity\n")
	if private {
		b.WriteString("tags: [git, repo, private]\n")
	} else {
		b.WriteString("tags: [git, repo]\n")
	}
	b.WriteString(fmt.Sprintf("aliases: [%s]\n", repoName))
	b.WriteString(fmt.Sprintf("git_url: %s\n", spec))
	b.WriteString(fmt.Sprintf("git_default_branch: %s\n", branch))
	b.WriteString(fmt.Sprintf("git_sha: %s\n", sha))
	b.WriteString(fmt.Sprintf("last_ingested: %s\n", nowISO))
	b.WriteString("---\n\n")
	b.WriteString(fmt.Sprintf("Repository `%s` ingested from `%s`.\n\n", repoName, spec))
	b.WriteString("## Sources\n\n")
	for _, rel := range currentFiles {
		b.WriteString(fmt.Sprintf("- [[%s]]\n", slugMap[rel]))
	}
	tmp := fmt.Sprintf("%s.tmp.%d", path, os.Getpid())
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// RunError pairs a bash-equivalent exit code with a human message.
type RunError struct {
	Code int
	Msg  string
}

func (e *RunError) Error() string { return e.Msg }
func (e *RunError) ExitCode() int { return e.Code }
