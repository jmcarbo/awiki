package formats

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"awiki/internal/ingest"
)

// AudioOptions describes one `awiki ingest-audio <path>` invocation.
//
// SourcePath is the user-supplied path to the audio file. It must be
// relative to RepoRoot and rooted at `raw/inbox/` (mirroring the bash
// guard in scripts/ingest-audio.sh:8). Absolute paths under RepoRoot
// are also accepted and normalized to repo-relative form.
type AudioOptions struct {
	SourcePath string
}

// AudioResult is what the verb returns to the caller. DestPath and
// OriginalPath are repo-relative, matching the form bash emits in the
// AUDIO-TRANSCRIBED record.
type AudioResult struct {
	SourcePath   string
	DestPath     string
	OriginalPath string
}

// IngestAudio ports `scripts/ingest-audio.sh` to Go. The flow mirrors
// the bash byte-for-byte:
//
//  1. Validate the source exists and lives under raw/inbox/.
//  2. Compute the sibling .md output path: `${AUDIO%.*}.md`.
//  3. Copy the source into raw/processed/_originals/.
//  4. Transcribe via the injected Whisper adapter.
//  5. Prepend the canonical frontmatter (title / date / last_updated /
//     type / tags=[audio] / aliases / sources / original / draft) and
//     write the result to the destination path.
//  6. Remove the source from the inbox.
//  7. Emit two stdout records, byte-identical with bash:
//
//     AUDIO-TRANSCRIBED|out=<dst>|orig=<orig>
//     Now run: just ingest <dst>
//
// Note that the AUDIO-TRANSCRIBED record has no `in=` field — bash
// (scripts/ingest-audio.sh:31) emits only `out=` and `orig=`. Do not
// add an `in=` field "for symmetry" with PDF-CONVERTED; parity with
// bash is the contract.
//
// On adapter failure the source audio is left untouched and an
// ingest.ExitError{Code: 2} is returned (extractor failure exit code).
func IngestAudio(ctx context.Context, r *ingest.Runner, opts AudioOptions, stdout, stderr io.Writer) (*AudioResult, error) {
	src := opts.SourcePath
	if src == "" {
		fmt.Fprintln(stderr, "usage: ingest-audio.sh <audio-path-under-raw/inbox/>")
		return nil, &ingest.ExitError{Code: 1, Msg: "usage"}
	}

	if filepath.IsAbs(src) {
		if rel, err := filepath.Rel(r.RepoRoot, src); err == nil && !strings.HasPrefix(rel, "..") {
			src = filepath.ToSlash(rel)
		}
	} else {
		src = filepath.ToSlash(filepath.Clean(src))
	}

	abs := filepath.Join(r.RepoRoot, src)
	if info, err := os.Stat(abs); err != nil || info.IsDir() {
		fmt.Fprintf(stderr, "not found: %s\n", src)
		return nil, &ingest.ExitError{Code: 1, Msg: "not found"}
	}
	if !strings.HasPrefix(src, "raw/inbox/") {
		fmt.Fprintln(stderr, "must be under raw/inbox/")
		return nil, &ingest.ExitError{Code: 1, Msg: "must be under raw/inbox/"}
	}

	// Compute sibling .md path. Bash uses `${AUDIO%.*}.md` (strip any
	// extension). filepath.Ext returns the trailing extension including
	// the dot; for files with no extension this is empty and we just
	// append .md.
	ext := filepath.Ext(src)
	dst := strings.TrimSuffix(src, ext) + ".md"
	dstAbs := filepath.Join(r.RepoRoot, dst)

	origDir := "raw/processed/_originals"
	origName := filepath.Base(src)
	orig := origDir + "/" + origName
	origAbs := filepath.Join(r.RepoRoot, orig)
	if err := os.MkdirAll(filepath.Join(r.RepoRoot, origDir), 0o755); err != nil {
		return nil, fmt.Errorf("ingest-audio: mkdir originals: %w", err)
	}
	if err := copyFile(abs, origAbs); err != nil {
		return nil, fmt.Errorf("ingest-audio: stash original: %w", err)
	}

	if r.Whisper == nil {
		return nil, fmt.Errorf("ingest-audio: Whisper adapter is nil")
	}
	text, code, err := r.Whisper.Transcribe(ctx, abs)
	if err != nil || code != 0 {
		fmt.Fprintf(stderr, "ingest-audio: whisper failed (exit %d): %v\n", code, err)
		return nil, &ingest.ExitError{Code: 2, Msg: "whisper failed"}
	}

	today := r.Today
	if today == "" {
		today = todayFallback(r)
	}
	baseTitle := filepath.Base(src) // bash: $(basename "$AUDIO") — keeps extension
	fm := buildAudioFrontmatter(baseTitle, today, orig)
	body := fm + text

	if err := os.MkdirAll(filepath.Dir(dstAbs), 0o755); err != nil {
		return nil, fmt.Errorf("ingest-audio: mkdir dest: %w", err)
	}
	if err := os.WriteFile(dstAbs, []byte(body), 0o644); err != nil {
		return nil, fmt.Errorf("ingest-audio: write dest: %w", err)
	}

	if err := os.Remove(abs); err != nil {
		return nil, fmt.Errorf("ingest-audio: remove source: %w", err)
	}

	// scripts/ingest-audio.sh:31-32. Note: no `in=` field.
	fmt.Fprintf(stdout, "AUDIO-TRANSCRIBED|out=%s|orig=%s\n", dst, orig)
	fmt.Fprintf(stdout, "Now run: just ingest %s\n", dst)

	return &AudioResult{
		SourcePath:   src,
		DestPath:     dst,
		OriginalPath: orig,
	}, nil
}

// buildAudioFrontmatter mirrors the bash printf at
// scripts/ingest-audio.sh:25-26 byte-for-byte. Differs from the PDF
// frontmatter only in the tags line.
func buildAudioFrontmatter(title, today, orig string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("title: \"%s\"\n", title))
	b.WriteString(fmt.Sprintf("date: %s\n", today))
	b.WriteString(fmt.Sprintf("last_updated: %s\n", today))
	b.WriteString("type: source\n")
	b.WriteString("tags: [audio]\n")
	b.WriteString("aliases: []\n")
	b.WriteString("sources: []\n")
	b.WriteString(fmt.Sprintf("original: %s\n", orig))
	b.WriteString("draft: false\n")
	b.WriteString("---\n\n")
	return b.String()
}
