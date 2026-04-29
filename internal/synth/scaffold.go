package synth

import (
	"strings"
)

// ScaffoldInput holds the inputs for WriteScaffold.
type ScaffoldInput struct {
	Topic       string
	Plugin      string
	Today       string // date YYYY-MM-DD
	ScopeBlock  string // full scope: ... block, ending with newline
	SourcesYAML string // e.g. "sources: []\n"
	ScopeHash   string
}

// WriteScaffold returns the page bytes the bash
// synth_write_scaffold heredoc emits, byte-equivalent except for
// time-dependent fields the caller supplies.
func WriteScaffold(in ScaffoldInput) []byte {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(`title: "` + in.Topic + ` — ` + in.Plugin + `"` + "\n")
	b.WriteString("date: " + in.Today + "\n")
	b.WriteString("last_updated: " + in.Today + "\n")
	b.WriteString("last_generated:\n")
	b.WriteString("type: synthesis\n")
	b.WriteString("plugin: " + in.Plugin + "\n")
	b.WriteString(in.ScopeBlock)
	b.WriteString("tags: [" + in.Topic + ", " + in.Plugin + "]\n")
	b.WriteString("aliases: []\n")
	b.WriteString(in.SourcesYAML)
	b.WriteString("draft: false\n")
	b.WriteString("---\n")
	b.WriteString("\n")
	b.WriteString("Lead paragraph — written once by user/agent, NOT regenerated.\n")
	b.WriteString("\n")
	b.WriteString("## Notes\n")
	b.WriteString("\n")
	b.WriteString("<!-- user notes; survives regen -->\n")
	b.WriteString("\n")
	b.WriteString("<!-- BEGIN GENERATED plugin=" + in.Plugin +
		" scope_hash=" + in.ScopeHash + " -->\n")
	b.WriteString("\n")
	b.WriteString("<!-- END GENERATED -->\n")
	return []byte(b.String())
}
