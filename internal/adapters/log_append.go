package adapters

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LogAppend writes a journal entry to the awiki log. Historically this
// shelled scripts/log-append.sh; the bash script has now been deleted and
// the adapter implements the same byte-level append directly in Go so
// existing callers (internal/ingest, etc.) keep their interface.
//
// Records mirror scripts/log-append.sh:
//
//	## [YYYY-MM-DD HH:MM] <action> | <message>
//
// with the same '|' → '_' and '\n','\r' → ' ' sanitisation, and the
// canonical frontmatter when the file is missing.
type LogAppend interface {
	Append(ctx context.Context, repoRoot, action, msg string) error
}

// ExecLogAppend is the production adapter — writes the log directly.
type ExecLogAppend struct{}

func (ExecLogAppend) Append(_ context.Context, repoRoot, action, msg string) error {
	logFile := os.Getenv("AWIKI_LOG_FILE")
	if logFile == "" {
		logFile = filepath.Join(repoRoot, "content", "log.md")
	}
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		header := "---\ntitle: Log\ntype: log\ndraft: true\n---\n\n"
		if err := os.MkdirAll(filepath.Dir(logFile), 0o755); err != nil {
			return nil
		}
		if err := os.WriteFile(logFile, []byte(header), 0o644); err != nil {
			return nil
		}
	}
	f, err := os.OpenFile(logFile, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		// Bash form swallows errors; mirror that.
		return nil
	}
	defer f.Close()
	stamp := time.Now().Format("2006-01-02 15:04")
	_, _ = fmt.Fprintf(f, "## [%s] %s | %s\n", stamp, sanitiseLogField(action), sanitiseLogField(msg))
	return nil
}

// sanitiseLogField replaces '|' with '_' and CR/LF with space.
func sanitiseLogField(s string) string {
	s = strings.ReplaceAll(s, "|", "_")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}
