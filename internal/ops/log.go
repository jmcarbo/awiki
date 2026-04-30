// Package ops implements the awiki operational verbs (log, reindex,
// check-deps, rename, delete, update-catalog, scan, recur, agenda,
// review). Each verb mirrors the byte-level contract of its bash
// oracle under scripts/.
package ops

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LogOptions controls a `awiki log` invocation. Mirrors
// scripts/log-append.sh (action+message → ${AWIKI_LOG_FILE:-content/log.md}).
type LogOptions struct {
	// Action is the first positional arg (e.g. "ingest", "rename").
	Action string
	// Message is the remaining positional args, space-joined. Bash form
	// uses `$*` so a multi-word message arrives space-joined too.
	Message string
	// LogFile is the destination path. Empty falls back to
	// $AWIKI_LOG_FILE or "content/log.md".
	LogFile string
	// Now overrides the timestamp source (testing). Empty uses time.Now.
	Now time.Time
}

// Log appends one journal line to the configured log file. Returns the
// resolved log file path on success.
//
// Bash contract (scripts/log-append.sh):
//   - sanitize action and message: '|' -> '_', '\n','\r' -> ' '
//   - if file does not exist, create it with the canonical frontmatter
//     "---\ntitle: Log\ntype: log\ndraft: true\n---\n\n"
//   - append a single line: "## [YYYY-MM-DD HH:MM] <action> | <message>\n"
func Log(opts LogOptions) (string, error) {
	if opts.Action == "" {
		return "", fmt.Errorf("action required")
	}
	logFile := opts.LogFile
	if logFile == "" {
		if env := os.Getenv("AWIKI_LOG_FILE"); env != "" {
			logFile = env
		} else {
			logFile = filepath.Join("content", "log.md")
		}
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	stamp := now.Format("2006-01-02 15:04")
	action := sanitizeLogField(opts.Action)
	msg := sanitizeLogField(opts.Message)

	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		// Bash form uses `printf -- "---\ntitle: Log\ntype: log\ndraft: true\n---\n\n"`.
		header := "---\ntitle: Log\ntype: log\ndraft: true\n---\n\n"
		if err := os.MkdirAll(filepath.Dir(logFile), 0o755); err != nil {
			return logFile, err
		}
		if err := os.WriteFile(logFile, []byte(header), 0o644); err != nil {
			return logFile, err
		}
	} else if err != nil {
		return logFile, err
	}

	f, err := os.OpenFile(logFile, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return logFile, err
	}
	defer f.Close()
	line := fmt.Appendf(nil, "## [%s] %s | %s\n", stamp, action, msg)
	if _, err := f.Write(line); err != nil {
		return logFile, err
	}
	return logFile, nil
}

// sanitizeLogField mirrors awiki_log_sanitize from log-append.sh:
//   - '|' -> '_'
//   - '\n','\r' -> ' '
func sanitizeLogField(s string) string {
	s = strings.ReplaceAll(s, "|", "_")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}

// LogCLI parses the `awiki log <action> [message...]` argv shape and
// runs Log. Writes nothing to stdout/stderr on success (mirrors bash);
// returns non-zero with usage on missing args.
func LogCLI(args []string, _ io.Writer, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "usage: awiki log <action> [message...]")
		return 1
	}
	action := args[0]
	msg := strings.Join(args[1:], " ")
	if _, err := Log(LogOptions{Action: action, Message: msg}); err != nil {
		fmt.Fprintf(stderr, "log: %v\n", err)
		return 1
	}
	return 0
}
