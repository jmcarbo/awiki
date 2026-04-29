package synth

import (
	"strings"
	"testing"
)

func TestWriteScaffoldGolden(t *testing.T) {
	in := ScaffoldInput{
		Topic:       "t1",
		Plugin:      "briefing",
		Today:       "2024-01-15",
		ScopeBlock:  "scope:\n  tag: t1\n",
		SourcesYAML: "sources: []\n",
		ScopeHash:   "abc123",
	}
	got := string(WriteScaffold(in))

	// Verify structure matches bash heredoc byte-for-byte.
	want := `---
title: "t1 — briefing"
date: 2024-01-15
last_updated: 2024-01-15
last_generated:
type: synthesis
plugin: briefing
scope:
  tag: t1
tags: [t1, briefing]
aliases: []
sources: []
draft: false
---

Lead paragraph — written once by user/agent, NOT regenerated.

## Notes

<!-- user notes; survives regen -->

<!-- BEGIN GENERATED plugin=briefing scope_hash=abc123 -->

<!-- END GENERATED -->
`
	if got != want {
		t.Errorf("WriteScaffold mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestWriteScaffoldContainsMarkers(t *testing.T) {
	in := ScaffoldInput{
		Topic:       "my-topic",
		Plugin:      "myplug",
		Today:       "2024-03-01",
		ScopeBlock:  "scope:\n  tag: x\n",
		SourcesYAML: "sources: []\n",
		ScopeHash:   "deadbe",
	}
	got := string(WriteScaffold(in))

	checks := []string{
		"<!-- BEGIN GENERATED plugin=myplug scope_hash=deadbe -->",
		"<!-- END GENERATED -->",
		`title: "my-topic — myplug"`,
		"type: synthesis",
		"plugin: myplug",
		"draft: false",
	}
	for _, c := range checks {
		if !strings.Contains(got, c) {
			t.Errorf("missing %q in scaffold output", c)
		}
	}
}
