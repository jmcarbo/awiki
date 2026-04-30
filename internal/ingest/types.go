// Package ingest implements the awiki ingest domain commands
// (awiki ingest|ingest-xlsx|ingest-git|ingest-pdf|ingest-audio|capture|
// watchdog|ingest-batch-list|ingest-git-list).
package ingest

// Format identifies the source format for an ingest run. The zero value
// FormatUnknown means "auto-detect from extension"; concrete values are
// pinned by the per-format workers added in subsequent slices.
type Format string

const (
	FormatUnknown Format = ""
	FormatXLSX    Format = "xlsx"
	FormatGit     Format = "git"
	FormatPDF     Format = "pdf"
	FormatAudio   Format = "audio"
	FormatMD      Format = "md"
)

// Spec describes a single ingest invocation. Source is the user-supplied
// path or git spec; AgentCLI is non-empty when the caller asked to shell
// the agent (mirrors `--agent=<cli>` from `scripts/ingest.sh`). Subsequent
// slices fill in per-format flags.
type Spec struct {
	Source   string
	Format   Format
	AgentCLI string
	// PassFlags carries verb-specific pass-through flags (e.g. xlsx2csv
	// flags forwarded by `ingest-xlsx`). Subsequent slices wire these.
	PassFlags []string
}

// Result is the outcome of an ingest invocation. DestPath is the wiki
// page (or directory) the source was promoted to; Archive is the
// `raw/inbox/_archive/...` location of the source after bookkeeping.
// Subsequent slices fill in the fields they need.
type Result struct {
	Format   Format
	Source   string
	DestPath string
	Archive  string
}

// AgentPromptArgs is the data passed to the AGENT-PROMPT template
// emitted by `awiki ingest`. The byte format of the emitted record is
// pinned by golden fixture in slice 7; subsequent slices fill in the
// fields the template needs.
type AgentPromptArgs struct {
	Source   string
	DestPath string
	Format   Format
}
