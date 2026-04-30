package ingest

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"awiki/internal/fsutil"
)

// CaptureOptions describes one `awiki capture` invocation. Mirrors the
// argument shape of scripts/capture.sh: Text is the joined text after
// the `--` separator; Presanitized maps to AWIKI_CAPTURE_PRESANITIZED=1
// (caller already ran sanitization, e.g. the MCP capture handler);
// InboxPath overrides the default `<repo>/content/inbox.md` and maps to
// AWIKI_INBOX_FILE.
type CaptureOptions struct {
	Text         string
	Presanitized bool
	InboxPath    string
}

// captureTimeFormat matches the bash `date '+%Y-%m-%d %H:%M'` form.
const captureTimeFormat = "2006-01-02 15:04"

// captureBlockIDRe matches the bash `\^[a-z0-9]{4,}` rule used to
// identify block-ID-shaped tokens that need backslash-escaping.
var captureBlockIDRe = regexp.MustCompile(`\^[a-z0-9]{4,}`)

// captureCheckboxRe matches a leading checkbox marker at column 1:
// "[ ]", "[/]", "[?]", "[>]", "[x]", "[-]". Mirrors the bash test
// `^\[[\ /\?\>x-]\]`.
var captureCheckboxRe = regexp.MustCompile(`^\[[\ /\?\>x\-]\]`)

// inboxFrontmatter is the scaffold the bash script writes when
// content/inbox.md does not yet exist. Byte-identical with capture.sh.
const inboxFrontmatter = "---\ntitle: \"Inbox\"\ntype: inbox\ndraft: true\n---\n"

// Capture appends a sanitized capture line to the inbox file. It mirrors
// scripts/capture.sh byte-for-byte:
//   - hard-rejects control chars (exit 4)
//   - hard-rejects checkbox-prefix-at-start (exit 4)
//   - normalizes tab to space
//   - when Presanitized is false, applies length / wikilink / HTML
//     comment / block-ID sanitizations and emits SANITIZATION-APPLIED
//   - creates the inbox with a frontmatter scaffold if missing
//   - emits OK|appended|<line> on stdout
//
// scripts/capture.sh has no file lock (no scripts/lib/lock.sh source);
// we mirror that — concurrent captures are allowed and the bash writer
// already relies on `>>` append semantics. The file write is still
// atomic: when the inbox already exists we read-modify-write via
// fsutil.AtomicWrite. When it does not, we create-and-write directly.
//
// TODO(slice 7+ ops port): the bash script best-effort logs to
// scripts/log-append.sh after a successful capture. The Go ops package
// does not exist yet; restore the parity log call once it lands.
func (r *Runner) Capture(opts CaptureOptions, stdout, stderr io.Writer) error {
	raw := opts.Text
	if raw == "" {
		fmt.Fprintln(stderr, "ERROR|empty: text is blank after argument join")
		return &ExitError{Code: 4, Msg: "empty"}
	}

	// Hard-rejection: control characters. Tab (0x09) is allowed and
	// normalized below. Mirrors the byte set in capture.sh:54.
	for _, c := range []byte(raw) {
		if (c >= 0x01 && c <= 0x08) || (c >= 0x0a && c <= 0x1f) || c == 0x7f {
			fmt.Fprintln(stderr, "ERROR|control: text contains a newline / CR / NUL / control char (only tab is allowed and converts to space)")
			return &ExitError{Code: 4, Msg: "control"}
		}
	}

	// Hard-rejection: checkbox marker at column 1.
	if captureCheckboxRe.MatchString(raw) {
		fmt.Fprintln(stderr, "ERROR|checkbox: line begins with a checkbox marker; captures are not actions")
		return &ExitError{Code: 4, Msg: "checkbox"}
	}

	// Tab -> space (always; not counted as a sanitization).
	text := strings.ReplaceAll(raw, "\t", " ")

	var applied []string
	if !opts.Presanitized {
		// Length > 2000 -> truncate + ellipsis. Use rune count to match
		// the bash `${#text}` semantics, which counts characters under
		// LANG=*.UTF-8. Bash on the user's locale typically counts
		// runes; we mirror that here.
		runes := []rune(text)
		if len(runes) > 2000 {
			text = string(runes[:2000]) + "…"
			applied = append(applied, "length-truncated")
		}

		// Wikilink open/close.
		if strings.Contains(text, "[[") || strings.Contains(text, "]]") {
			text = strings.ReplaceAll(text, "[[", "[ [")
			text = strings.ReplaceAll(text, "]]", "] ]")
			applied = append(applied, "wikilink-neutralized")
		}

		// HTML comment markers.
		if strings.Contains(text, "<!--") || strings.Contains(text, "-->") {
			text = strings.ReplaceAll(text, "<!--", "< !--")
			text = strings.ReplaceAll(text, "-->", "--  >")
			applied = append(applied, "comment-neutralized")
		}

		// Block-ID-shaped tokens: ^[a-z0-9]{4,} -> \^[a-z0-9]{4,}.
		if captureBlockIDRe.MatchString(text) {
			text = captureBlockIDRe.ReplaceAllStringFunc(text, func(m string) string {
				return "\\" + m
			})
			applied = append(applied, "block-id-escaped")
		}
	}

	// Resolve inbox path.
	inbox := opts.InboxPath
	if inbox == "" {
		inbox = filepath.Join(r.RepoRoot, "content", "inbox.md")
	} else if !filepath.IsAbs(inbox) {
		inbox = filepath.Join(r.RepoRoot, inbox)
	}
	if err := os.MkdirAll(filepath.Dir(inbox), 0o755); err != nil {
		return fmt.Errorf("capture: mkdir inbox parent: %w", err)
	}

	ts := r.captureNow().Format(captureTimeFormat)
	line := "- " + ts + " " + text

	if err := appendInboxLine(inbox, line); err != nil {
		return fmt.Errorf("capture: write inbox: %w", err)
	}

	if len(applied) > 0 {
		fmt.Fprintf(stderr, "SANITIZATION-APPLIED|%s\n", strings.Join(applied, ","))
		fmt.Fprintf(stderr, "  raw  : %s\n", raw)
		fmt.Fprintf(stderr, "  final: %s\n", text)
	}

	fmt.Fprintf(stdout, "OK|appended|%s\n", line)
	return nil
}

// appendInboxLine appends `line\n` to inbox. If the file does not exist
// it is created with the canonical frontmatter scaffold first. The
// existing-file path uses fsutil.AtomicWrite for crash safety; the
// create path uses os.WriteFile (atomic-by-rename is unnecessary on
// initial create — there is no prior content to corrupt).
func appendInboxLine(inbox, line string) error {
	existing, err := os.ReadFile(inbox)
	if os.IsNotExist(err) {
		body := inboxFrontmatter + line + "\n"
		return os.WriteFile(inbox, []byte(body), 0o644)
	}
	if err != nil {
		return err
	}
	// Ensure the file ends in a newline before appending, matching the
	// bash `printf '%s\n' >>` append semantics.
	body := string(existing)
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	body += line + "\n"
	return fsutil.AtomicWrite(inbox, []byte(body))
}

// captureNow returns the timestamp source for the capture line. Tests
// inject a fixed time via Runner.NowFn; production reads time.Now.
func (r *Runner) captureNow() time.Time {
	if r.NowFn != nil {
		return r.NowFn()
	}
	return time.Now()
}
